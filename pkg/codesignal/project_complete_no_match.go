package codesignal

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
	if hasActiveLayerFinding(projectChanges) {
		return false
	}
	if !opts.Baseline && hasPreFilterProjectChanges(projectSummary) {
		return false
	}
	return true
}

func hasActiveLayerFinding(projectChanges []ProjectChange) bool {
	for _, ch := range projectChanges {
		if ch.RuleID == ruleLayerViolationID || ch.RuleID == ruleLayerBypassID {
			return true
		}
	}
	return false
}

func hasPreFilterProjectChanges(summary *ProjectSummary) bool {
	return summary != nil && (summary.IntroducedChanges > 0 || summary.ExistingChanges > 0 || summary.ResolvedChanges > 0)
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
