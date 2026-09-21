package codesignal

import "github.com/lousy-agents/coach/pkg/projectmodel"

// mapCoveragePhase converts a phase-specific Coverage observation to one of
// the four frozen D7 vocabulary strings: complete, incomplete, not_run,
// not_requested.
//
// scopeNil must be true when the corresponding side's ProjectScope is nil
// (the analyzer never produced root_scopes for that revision). When scopeNil
// is true and the coverage carries DiagBackendUnavailable, the outcome is
// "not_run" rather than "incomplete", distinguishing "never ran" from
// "ran but found coverage gaps".
func mapCoveragePhase(cov *projectmodel.Coverage, scopeNil bool) string {
	if cov == nil {
		return "not_run"
	}
	if cov.Phase == "not_requested" {
		return "not_requested"
	}
	if scopeNil && hasDiagCode(cov, projectmodel.DiagBackendUnavailable) {
		return "not_run"
	}
	if cov.Complete {
		return "complete"
	}
	return "incomplete"
}

func hasDiagCode(cov *projectmodel.Coverage, code string) bool {
	for _, d := range cov.Diagnostics {
		if d.Code == code {
			return true
		}
	}
	return false
}

// buildProjectProvenance constructs the ProjectProvenance for a report.
// Returns nil when Options.ProjectEnabled is false or input.Language is not
// "typescript" (Go schema-2 reports omit provenance).
func buildProjectProvenance(input Input, opts Options) *ProjectProvenance {
	if !opts.ProjectEnabled || input.Language != "typescript" {
		return nil
	}
	headScopeNil := input.HeadProjectScope == nil
	headCov := ProvenanceCoverage{
		Model:        mapCoveragePhase(input.HeadModelCoverage, headScopeNil),
		Bypass:       mapCoveragePhase(input.HeadBypassCoverage, headScopeNil),
		Reachability: mapCoveragePhase(input.HeadReachabilityCoverage, headScopeNil),
	}
	analyzerProtocolVersion := input.AnalyzerProtocolVersion
	if analyzerProtocolVersion == 0 {
		analyzerProtocolVersion = 1
	}
	prov := &ProjectProvenance{
		Language:      "typescript",
		ConfigDigest:  input.ConfigDigest,
		SelectedRoots: input.SelectedRoots,
		Analyzer: ProvenanceAnalyzer{
			ProtocolVersion: analyzerProtocolVersion,
		},
		Runtime: ProvenanceRuntime{
			Kind:            input.RuntimeKind,
			Version:         input.RuntimeVersion,
			Origin:          input.RuntimeOrigin,
			CompilerVersion: input.CompilerVersion,
			CompilerOrigin:  input.CompilerOrigin,
		},
		Head: ProvenanceRevision{
			Revision: input.Scope.Revision,
			Coverage: headCov,
		},
	}
	if input.ProjectBaseAnalyzed {
		baseScopeNil := input.BaseProjectScope == nil
		prov.Base = &ProvenanceRevision{
			Revision: input.Scope.Base,
			Coverage: ProvenanceCoverage{
				Model:        mapCoveragePhase(input.BaseModelCoverage, baseScopeNil),
				Bypass:       mapCoveragePhase(input.BaseBypassCoverage, baseScopeNil),
				Reachability: mapCoveragePhase(input.BaseReachabilityCoverage, baseScopeNil),
			},
		}
	}
	return prov
}

// buildProjectScope constructs the ProjectScopeReport from Input's scope
// fields. Returns nil when Language is not "typescript" (Go schema-2 reports
// omit it, mirroring the buildProjectProvenance gate) or when HeadProjectScope
// is nil (analyzer never produced root_scopes, so there is nothing to report).
func buildProjectScope(input Input) *ProjectScopeReport {
	if input.Language != "typescript" {
		return nil
	}
	if input.HeadProjectScope == nil {
		return nil
	}
	s := &ProjectScopeReport{
		InclusionRule: input.HeadProjectScope.InclusionRule,
		PatternSet:    input.HeadProjectScope.PatternSet,
		Head: ProjectScopeRevisionReport{
			Revision:        input.Scope.Revision,
			Roots:           input.HeadProjectScope.Roots,
			MatchedLayers:   input.HeadProjectScope.MatchedLayers,
			UnmatchedLayers: input.HeadProjectScope.UnmatchedLayers,
		},
	}
	if input.ProjectBaseAnalyzed && input.BaseProjectScope != nil {
		s.Base = &ProjectScopeRevisionReport{
			Revision:        input.Scope.Base,
			Roots:           input.BaseProjectScope.Roots,
			MatchedLayers:   input.BaseProjectScope.MatchedLayers,
			UnmatchedLayers: input.BaseProjectScope.UnmatchedLayers,
		}
	}
	return s
}

// isRequiredCoverageComplete reports whether required project coverage is
// complete for all analyzed sides. Required coverage tracks model and bypass
// only; reachability incompleteness alone does not block completeness.
//
// A side's coverage is required-complete when its model phase is "complete"
// and its bypass phase is "complete" or "not_requested".
func isRequiredCoverageComplete(headModel, headBypass, baseModel, baseBypass string, projectBaseAnalyzed bool) bool {
	headOK := headModel == "complete" && (headBypass == "complete" || headBypass == "not_requested")
	if !headOK {
		return false
	}
	if !projectBaseAnalyzed {
		return true
	}
	return baseModel == "complete" && (baseBypass == "complete" || baseBypass == "not_requested")
}

// isCompleteNoMatch reports whether the complete_no_match predicate holds:
// required coverage is complete, no active layer findings, and (in diff mode)
// zero introduced/resolved/existing project changes in the pre-filter summary.
//
// projectSummary carries lifecycle counts before IncludeResolved filtering, so
// a resolved change that was removed from the active set still blocks the
// predicate. projectChanges is the active (post-filter) set used to check for
// active layer-rule findings.
func isCompleteNoMatch(opts Options, prov *ProjectProvenance, input Input, projectSummary *ProjectSummary, projectChanges []ProjectChange) bool {
	if prov == nil {
		return false
	}
	var baseModel, baseBypass string
	if prov.Base != nil {
		baseModel = prov.Base.Coverage.Model
		baseBypass = prov.Base.Coverage.Bypass
	}
	if !isRequiredCoverageComplete(
		prov.Head.Coverage.Model, prov.Head.Coverage.Bypass,
		baseModel, baseBypass,
		input.ProjectBaseAnalyzed,
	) {
		return false
	}
	for _, ch := range projectChanges {
		if ch.RuleID == ruleLayerViolationID || ch.RuleID == ruleLayerBypassID {
			return false
		}
	}
	if !opts.Baseline {
		if projectSummary != nil && (projectSummary.IntroducedChanges > 0 || projectSummary.ExistingChanges > 0 || projectSummary.ResolvedChanges > 0) {
			return false
		}
	}
	return true
}

// buildNextActions constructs the project_next_actions list when
// complete_no_match holds. Returns nil when completeNoMatch is false (the key
// is omitted from the report via omitempty).
func buildNextActions(opts Options, completeNoMatch bool, diagnostics []Diagnostic) []ProjectNextAction {
	if !completeNoMatch {
		return nil
	}
	var actions []ProjectNextAction
	if opts.Baseline {
		actions = append(actions, ProjectNextAction{Kind: "record_baseline"})
	}
	actions = append(actions, ProjectNextAction{Kind: "review_policy_coverage"})
	if len(diagnostics) > 0 {
		actions = append(actions, ProjectNextAction{Kind: "inspect_diagnostics"})
	}
	return actions
}
