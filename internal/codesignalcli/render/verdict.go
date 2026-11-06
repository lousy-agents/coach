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
		renderCoveredNoMatchVerdict(b, report)
		return
	}

	incompleteProject := (report.ProjectCoverage != nil && !report.ProjectCoverage.Complete) ||
		codesignal.HasProjectCoverageDiagnostic(report.Diagnostics)
	if keepUnqualifiedAllClear(report, incompleteProject) {
		fmt.Fprintf(b, "%s%s.\n", noActiveFindingsLead(report), layerReachSuffix(report))
		return
	}

	var causes []string
	if n := report.Summary.FilesUnanalyzed; n > 0 {
		causes = append(causes, pathCountClause(n))
	}
	if incompleteProject {
		causes = append(causes, "project analysis did not complete")
	}
	causes = append(causes, unplacedImportClauses(report.Diagnostics)...)
	causes = append(causes, worktreeScanClauses(report.Diagnostics)...)
	if len(causes) == 0 {
		causes = append(causes, "additional diagnostics were recorded")
	}
	fmt.Fprintf(b, "%s, but the analysis is incomplete: %s.%s\n", noActiveFindingsLead(report), strings.Join(causes, "; "), layerReachSentence(report))
}

func renderCoveredNoMatchVerdict(b *strings.Builder, report *codesignal.Report) {
	unanalyzed := report.Summary.FilesUnanalyzed
	if unplaced := unplacedImportClauses(report.Diagnostics); len(unplaced) > 0 {
		causes := unplaced
		if unanalyzed > 0 {
			causes = append([]string{pathCountClause(unanalyzed)}, causes...)
		}
		fmt.Fprintf(b, "Scan found no configured covered match, but the analysis is incomplete: %s.%s\n", strings.Join(causes, "; "), layerReachSentence(report))
		return
	}
	if unanalyzed == 0 {
		fmt.Fprintf(b, "Complete scan found no configured covered match%s.\n", layerReachSuffix(report))
		return
	}
	fmt.Fprintf(b, "Complete scan found no configured covered match, but %s.%s\n", pathCountClause(unanalyzed), layerReachSentence(report))
}

// noActiveFindingsLead scopes the all-clear to the severity floor: when a
// floor withheld signals, "no active findings" would claim more than the
// narrowed view can show.
func noActiveFindingsLead(report *codesignal.Report) string {
	if withheld := report.SignalsWithheld; withheld != nil && withheld.BelowMinSeverity > 0 {
		return "No active CodeSignal findings at or above --min-severity " + string(withheld.MinSeverity)
	}
	return "No active CodeSignal findings"
}

// worktreeScanClauses carries each run-level cause a --worktree scan recorded
// for paths it cannot see, so the verdict names it.
func worktreeScanClauses(diagnostics []codesignal.Diagnostic) []string {
	var clauses []string
	for _, d := range diagnostics {
		if d.Kind == codesignal.DiagKindWorktreeScanIncomplete {
			clauses = append(clauses, d.Message)
		}
	}
	return clauses
}
