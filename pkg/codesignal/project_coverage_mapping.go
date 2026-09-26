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
	var packageManager *ProvenancePackageManager
	if input.PackageManagerKind != "" {
		packageManager = &ProvenancePackageManager{
			Kind:    input.PackageManagerKind,
			Version: input.PackageManagerVersion,
			Origin:  input.PackageManagerOrigin,
		}
	}

	prov := &ProjectProvenance{
		Language:       "typescript",
		ConfigDigest:   input.ConfigDigest,
		SelectedRoots:  input.SelectedRoots,
		PackageManager: packageManager,
		Analyzer: ProvenanceAnalyzer{
			Version:         input.AnalyzerVersion,
			Digest:          input.AnalyzerDigest,
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
