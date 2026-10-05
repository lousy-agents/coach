// Package render presents CodeSignal reports and project-readiness results as
// the CLI's text and JSON output.
package render

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// ReportText renders report as deterministic, ANSI-free plain text.
func ReportText(report *codesignal.Report) string {
	var b strings.Builder
	renderReportSummary(&b, report)
	renderProjectScopeSection(&b, report.ProjectScope)
	renderActiveFindings(&b, report)
	renderProjectFacts(&b, report.ProjectFacts)
	renderDiagnosticsSection(&b, report.Diagnostics)
	renderCoverageSection(&b, report.Coverage)
	renderProjectCoverageSection(&b, report.ProjectCoverage)
	renderProjectProvenanceSection(&b, report.ProjectProvenance)
	renderProjectNextActionsSection(&b, report.ProjectNextActions)
	return b.String()
}

func renderActiveFindings(b *strings.Builder, report *codesignal.Report) {
	if len(report.Signals) == 0 && len(report.ProjectChanges) == 0 {
		renderNoActiveFindingsVerdict(b, report)
		renderProjectSummary(b, report.ProjectSummary)
		return
	}

	renderedSignal := renderFileLocalSignals(b, report.Signals, projectChangeSignalIDs(report.ProjectChanges))
	if len(report.ProjectChanges) > 0 {
		if renderedSignal {
			b.WriteString("\n")
		}
		renderProjectChanges(b, report)
		return
	}
	if report.ProjectSummary != nil {
		renderProjectSummary(b, report.ProjectSummary)
	}
}

func renderReportSummary(b *strings.Builder, report *codesignal.Report) {
	if report.Scope.Baseline {
		renderBaselineSummary(b, report)
		return
	}
	renderDiffSummary(b, report)
}

func renderDiffSummary(b *strings.Builder, report *codesignal.Report) {
	switch report.Scope.AppliedScope {
	case "all":
		fmt.Fprintf(b, "scope: all (no scope filtering applied), ")
	case "":
	default:
		var filtered int
		if report.Coverage != nil {
			filtered = sumCoverageGroups(report.Coverage.Excluded)
		}
		fmt.Fprintf(b, "scope: %s, filtered: %d, ", report.Scope.AppliedScope, filtered)
	}

	fmt.Fprintf(b, "files analyzed: %d, active signals: %d, diagnostics: %d\n",
		report.Summary.FilesAnalyzed, report.Summary.ActiveSignals, len(report.Diagnostics))
}

func renderBaselineSummary(b *strings.Builder, report *codesignal.Report) {
	fmt.Fprintf(b, "Repository Baseline for revision %s (not a diff comparison)\n", report.Scope.Revision)

	var tracked, analyzed, unanalyzable, unsupported, excluded int
	if report.Coverage != nil {
		tracked = report.Coverage.TrackedFilesDiscovered
		analyzed = report.Coverage.FilesAnalyzed
		unanalyzable = report.Coverage.FilesUnanalyzable
		unsupported = sumCoverageGroups(report.Coverage.Unsupported)
		excluded = sumCoverageGroups(report.Coverage.Excluded)
	}

	fmt.Fprintf(b, "tracked files discovered: %d, analyzed: %d, unsupported: %d, excluded: %d, unanalyzable: %d, active signals: %d, diagnostics: %d\n",
		tracked, analyzed, unsupported, excluded, unanalyzable, report.Summary.ActiveSignals, len(report.Diagnostics))
}

func pathCountClause(n int) string {
	if n == 1 {
		return "1 path was not analyzed"
	}
	return fmt.Sprintf("%d paths were not analyzed", n)
}

func sumCoverageGroups(groups []codesignal.CoverageGroup) int {
	total := 0
	for _, g := range groups {
		total += g.Count
	}
	return total
}
