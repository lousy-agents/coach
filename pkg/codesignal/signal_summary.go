package codesignal

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

func countFilesWithDiagnostics(_ []FileChange, diagnostics []Diagnostic) int {
	return len(distinctDiagnosticPaths(diagnostics))
}

func countUnanalyzedFiles(files []FileChange, diagnostics []Diagnostic) int {
	inFiles := make(map[string]bool, len(files))
	for _, fc := range files {
		if fc.Path != "" {
			inFiles[fc.Path] = true
		}
	}
	count := 0
	for path := range distinctDiagnosticPaths(diagnostics) {
		if !inFiles[path] {
			count++
		}
	}
	return count
}

func distinctDiagnosticPaths(diagnostics []Diagnostic) map[string]struct{} {
	paths := make(map[string]struct{})
	for _, d := range diagnostics {
		if d.Path != "" {
			paths[d.Path] = struct{}{}
		}
	}
	return paths
}
