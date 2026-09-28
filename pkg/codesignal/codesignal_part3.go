package codesignal

func projectLifecycleState(input Input) (indeterminate bool, diagnostics []Diagnostic) {

	if !completeProjectCoverage(input.ProjectCoverage) {
		indeterminate = true
	}
	if input.ProjectBaseAnalyzed && !completeProjectCoverage(input.BaseProjectCoverage) {
		indeterminate = true
	}

	if !input.ProjectBaseAnalyzed && len(input.BaseProjectChanges) > 0 {
		indeterminate = true
	}
	if input.ProjectCoverage != nil && !input.ProjectCoverage.Complete {
		diagnostics = append(diagnostics, Diagnostic{
			Kind:    DiagKindProjectCoverageIncomplete,
			Message: "project analysis coverage is incomplete; project observations may be partial",
		})
	}
	if indeterminate {
		diagnostics = append(diagnostics, Diagnostic{
			Kind:    DiagKindProjectLifecycleIndeterminate,
			Message: projectLifecycleDiagnosticMessage(input),
		})
	}
	return indeterminate, diagnostics
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
