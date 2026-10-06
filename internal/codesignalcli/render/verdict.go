package render

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// renderNoActiveFindingsVerdict qualifies the verdict from a diagnostic Kind,
// not only report.ProjectCoverage.Complete: Report exposes only head-side
// coverage, and a diff's base side can be incomplete while the head side is
// complete, which the Complete field alone cannot see.
func renderNoActiveFindingsVerdict(b *strings.Builder, report *codesignal.Report) {
	if len(report.ProjectNextActions) > 0 {
		if report.Summary.FilesUnanalyzed == 0 {
			b.WriteString("Complete scan found no configured covered match.\n")
			return
		}
		fmt.Fprintf(b, "Complete scan found no configured covered match, but %s.\n", pathCountClause(report.Summary.FilesUnanalyzed))
		return
	}

	incompleteProject := (report.ProjectCoverage != nil && !report.ProjectCoverage.Complete) ||
		hasProjectLifecycleDiagnostic(report.Diagnostics)
	if keepUnqualifiedAllClear(report, incompleteProject) {
		b.WriteString("No active CodeSignal findings.\n")
		return
	}

	var causes []string
	if n := report.Summary.FilesUnanalyzed; n > 0 {
		causes = append(causes, pathCountClause(n))
	}
	if incompleteProject {
		causes = append(causes, "project analysis did not complete")
	}
	if len(causes) == 0 {
		causes = append(causes, "additional diagnostics were recorded")
	}
	fmt.Fprintf(b, "No active CodeSignal findings, but the analysis is incomplete: %s.\n", strings.Join(causes, "; "))
}

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
