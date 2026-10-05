package codesignal

import "github.com/lousy-agents/coach/pkg/domain"

func projectLifecycleState(input Input) (state projectLifecycleIndeterminacy, diagnostics []Diagnostic) {
	state.headRevision = input.Scope.Revision
	state.baseRevision = input.Scope.Base

	if !completeProjectCoverage(input.ProjectCoverage) {
		state.headIncomplete = true
	}
	if input.ProjectBaseAnalyzed && !completeProjectCoverage(input.BaseProjectCoverage) {
		state.baseIncomplete = true
	}

	if !input.ProjectBaseAnalyzed && len(input.BaseProjectChanges) > 0 {
		state.inconsistentBase = true
	}
	if input.ProjectCoverage != nil && !input.ProjectCoverage.Complete {
		diagnostics = append(diagnostics, Diagnostic{
			Kind:    DiagKindProjectCoverageIncomplete,
			Message: "project analysis coverage is incomplete; project observations may be partial",
		})
	}
	if state.any() {
		diagnostics = append(diagnostics, Diagnostic{
			Kind:    DiagKindProjectLifecycleIndeterminate,
			Message: projectLifecycleDiagnosticMessage(input),
		})
	}
	return state, diagnostics
}

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
func finalizeSignals(signals []Signal, filesAnalyzed int, files []FileChange, diagnostics []Diagnostic, includeResolved bool) ([]Signal, Summary) {
	summary := Summary{
		FilesAnalyzed:        filesAnalyzed,
		FilesWithDiagnostics: countFilesWithDiagnostics(files, diagnostics),
		FilesUnanalyzed:      countUnanalyzedFiles(files, diagnostics),
	}
	for _, sig := range signals {
		switch sig.Lifecycle {
		case "introduced":
			summary.IntroducedSignals++
		case "existing":
			summary.ExistingSignals++
		case "resolved":
			summary.ResolvedSignals++
		case "baseline":
			summary.BaselineSignals++
		case "unknown":
			summary.UnknownSignals++
		}
	}
	if !includeResolved {
		signals = omitResolvedSignals(signals)
	}
	sortSignals(signals)
	summary.ActiveSignals = len(signals)
	return signals, summary
}

func omitResolvedSignals(signals []Signal) []Signal {
	filtered := make([]Signal, 0, len(signals))
	for _, sig := range signals {
		if sig.Lifecycle == "resolved" {
			continue
		}
		filtered = append(filtered, sig)
	}
	return filtered
}
