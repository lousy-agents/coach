package render

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func renderReportSummary(b *strings.Builder, report *codesignal.Report) {
	if report.Scope.Baseline {
		renderBaselineSummary(b, report)
		return
	}
	renderDiffSummary(b, report)
	renderWorktreeLine(b, report)
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
	if report.Scope.Worktree != nil {
		fmt.Fprintf(b, "Repository Baseline for revision %s with working-tree changes (not a diff comparison)\n", report.Scope.Revision)
		renderWorktreeLine(b, report)
	} else {
		fmt.Fprintf(b, "Repository Baseline for revision %s (not a diff comparison)\n", report.Scope.Revision)
	}

	var tracked, analyzed, unanalyzable, unsupported, excluded int
	if report.Coverage != nil {
		tracked = report.Coverage.TrackedFilesDiscovered
		analyzed = report.Coverage.FilesAnalyzed
		unanalyzable = report.Coverage.FilesUnanalyzable
		unsupported = sumCoverageGroups(report.Coverage.Unsupported)
		excluded = sumCoverageGroups(report.Coverage.Excluded)
	}

	fmt.Fprintf(b, "%s: %d, analyzed: %d, unsupported: %d, excluded: %d, unanalyzable: %d, active signals: %d, diagnostics: %d\n",
		discoveredLabel(report), tracked, analyzed, unsupported, excluded, unanalyzable, report.Summary.ActiveSignals, len(report.Diagnostics))
}

// discoveredLabel names what the discovered count covers. A --worktree scan
// also discovers untracked and staged new files, so "tracked" alone would
// misstate the number; the committed scan's label is unchanged.
func discoveredLabel(report *codesignal.Report) string {
	if report.Scope.Worktree != nil {
		return "files discovered (tracked and new)"
	}
	return "tracked files discovered"
}

func sumCoverageGroups(groups []codesignal.CoverageGroup) int {
	total := 0
	for _, g := range groups {
		total += g.Count
	}
	return total
}
