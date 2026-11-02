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
		if unresolved := countUnresolvedImportDiagnostics(report.Diagnostics); unresolved > 0 {
			causes := []string{unresolvedImportsClause(unresolved)}
			if n := report.Summary.FilesUnanalyzed; n > 0 {
				causes = append([]string{pathCountClause(n)}, causes...)
			}
			fmt.Fprintf(b, "Scan found no configured covered match, but the analysis is incomplete: %s.%s\n", strings.Join(causes, "; "), layerReachSentence(report))
			return
		}
		if report.Summary.FilesUnanalyzed == 0 {
			fmt.Fprintf(b, "Complete scan found no configured covered match%s.\n", layerReachSuffix(report))
			return
		}
		fmt.Fprintf(b, "Complete scan found no configured covered match, but %s.%s\n", pathCountClause(report.Summary.FilesUnanalyzed), layerReachSentence(report))
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
	if n := countUnresolvedImportDiagnostics(report.Diagnostics); n > 0 {
		causes = append(causes, unresolvedImportsClause(n))
	}
	if len(causes) == 0 {
		causes = append(causes, "additional diagnostics were recorded")
	}
	fmt.Fprintf(b, "%s, but the analysis is incomplete: %s.%s\n", noActiveFindingsLead(report), strings.Join(causes, "; "), layerReachSentence(report))
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
