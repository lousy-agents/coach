package codesignal

func hasDiagnosticKind(diagnostics []Diagnostic, kind string) bool {
	for _, d := range diagnostics {
		if d.Kind == kind {
			return true
		}
	}
	return false
}
