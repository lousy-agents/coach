package codesignal

import "github.com/lousy-agents/coach/pkg/domain"

// modelCoverageDiagnostics promotes each per-revision model-phase Coverage's
// own domain.DiagRootScopeIncomplete entries into report.Diagnostics[],
// stamped with the comparison side and that side's resolved revision.
func modelCoverageDiagnostics(input Input) []Diagnostic {
	diagnostics := sideModelCoverageDiagnostics(input.HeadModelCoverage, "head", input.Scope.Revision)
	if input.ProjectBaseAnalyzed {
		diagnostics = append(diagnostics, sideModelCoverageDiagnostics(input.BaseModelCoverage, "base", input.Scope.Base)...)
	}
	return diagnostics
}

// sideModelCoverageDiagnostics promotes only DiagRootScopeIncomplete entries:
// model coverage also carries codes that are not incompleteness (the
// ts_reachability_*_gap codes never flip Coverage.Complete), and promoting
// those would misreport a routine reachability gap as a side-attributed
// coverage failure. A base-side Kind is prefixed with "base_", mirroring
// internal/codesignalcli's baseProjectDiagnostics.
func sideModelCoverageDiagnostics(coverage *domain.Coverage, side, revision string) []Diagnostic {
	if coverage == nil {
		return nil
	}
	var diagnostics []Diagnostic
	for _, d := range coverage.Diagnostics {
		if d.Code != domain.DiagRootScopeIncomplete {
			continue
		}
		kind := d.Code
		if side == "base" {
			kind = "base_" + kind
		}
		diagnostics = append(diagnostics, Diagnostic{
			Path:     d.Path,
			Kind:     kind,
			Message:  sideRevisionLabel(side, revision) + " project model coverage: " + d.Message,
			Side:     side,
			Revision: revision,
		})
	}
	return diagnostics
}
