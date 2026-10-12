package render

import "github.com/lousy-agents/coach/pkg/codesignal"

func keepUnqualifiedAllClear(report *codesignal.Report, incompleteProject bool) bool {
	if incompleteProject {
		return false
	}
	if len(report.Diagnostics) == 0 {
		return true
	}
	return report.Summary.FilesUnanalyzed == 0 && onlyWorktreeCleanlinessDiagnostics(report.Diagnostics)
}

func onlyWorktreeCleanlinessDiagnostics(diagnostics []codesignal.Diagnostic) bool {
	if len(diagnostics) == 0 {
		return false
	}
	for _, d := range diagnostics {
		switch d.Kind {
		case codesignal.DiagKindWorktreeNotClean, codesignal.DiagKindWorktreeReportReflectsCommittedHEAD:
		default:
			return false
		}
	}
	return true
}
