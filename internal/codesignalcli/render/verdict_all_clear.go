package render

import (
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func keepUnqualifiedAllClear(report *codesignal.Report, incompleteProject bool) bool {
	if incompleteProject {
		return false
	}
	if len(report.Diagnostics) == 0 {
		return true
	}
	return report.Summary.FilesUnanalyzed == 0 && onlyInformationalDiagnostics(report.Diagnostics)
}

// onlyInformationalDiagnostics is a closed list: a diagnostic kind it does not
// name qualifies the verdict, so a new kind never silently reads as an
// all-clear. The not-evaluated count is informational because a package or a
// non-source file cannot match a path-prefix layer. It also counts an alias the
// sidecar does not follow (see docs/cli-codesignal.md), which can hide a
// forbidden edge; that gap is documented rather than qualifying every verdict
// of a project that uses such an alias.
func onlyInformationalDiagnostics(diagnostics []codesignal.Diagnostic) bool {
	if len(diagnostics) == 0 {
		return false
	}
	for _, d := range diagnostics {
		switch {
		case d.Kind == codesignal.DiagKindWorktreeNotClean, d.Kind == codesignal.DiagKindWorktreeReportReflectsCommittedHEAD:
		case strings.TrimPrefix(d.Kind, "base_") == codesignal.DiagKindProjectExternalImportsNotEvaluated:
		default:
			return false
		}
	}
	return true
}
