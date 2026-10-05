package codesignal

func processFileChanges(files []FileChange, seed []Diagnostic, noBaseLifecycle Lifecycle) ([]Diagnostic, []Signal) {
	diagnostics := make([]Diagnostic, 0, len(seed))
	diagnostics = append(diagnostics, seed...)
	var signals []Signal
	for _, fc := range files {
		diagnostics = append(diagnostics, validateFileChange(fc)...)

		fileDiagnostics, fileSignals := processHeadResult(fc)
		diagnostics = append(diagnostics, fileDiagnostics...)

		rangeDiagnostics, validRanges := validateChangedRanges(fc)
		diagnostics = append(diagnostics, rangeDiagnostics...)

		if !eligibleForLifecycleClassification(fc) {
			continue
		}
		fileClassifiedSignals := classifyFileSignals(baseUsableForLifecycle(fc), fileSignals, extractBaseSignals(fc), noBaseLifecycleForFile(fc, noBaseLifecycle))
		for i := range fileClassifiedSignals {
			fileClassifiedSignals[i].SourceScope = fc.SourceScope
		}
		signals = append(signals, markChanged(fileClassifiedSignals, validRanges)...)
	}
	return diagnostics, signals
}

func validateFileChange(fc FileChange) []Diagnostic {
	var diagnostics []Diagnostic

	if fc.Base != nil && fc.Base.Path != "" && fc.Base.Path != fc.Path {
		diagnostics = append(diagnostics, Diagnostic{
			Path:    fc.Path,
			Kind:    "invalid_file_change",
			Message: "base result path \"" + fc.Base.Path + "\" does not match file change path \"" + fc.Path + "\"",
		})
	}
	if fc.Head != nil && fc.Head.Path != "" && fc.Head.Path != fc.Path {
		diagnostics = append(diagnostics, Diagnostic{
			Path:    fc.Path,
			Kind:    "invalid_file_change",
			Message: "head result path \"" + fc.Head.Path + "\" does not match file change path \"" + fc.Path + "\"",
		})
	}

	return diagnostics
}

func baseUsableForLifecycle(fc FileChange) bool {
	if fc.Base == nil {
		return false
	}
	return fc.Base.Path == "" || fc.Base.Path == fc.Path
}

func eligibleForLifecycleClassification(fc FileChange) bool {
	if fc.Head != nil {
		return fc.Head.ParseStatus == "ok"
	}
	return fc.Status == "removed"
}
