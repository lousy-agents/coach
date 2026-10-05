package codesignalcli

import (
	"fmt"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"

	"strings"
)

func hasProjectLifecycleDiagnostic(diagnostics []codesignal.Diagnostic) bool {
	for _, d := range diagnostics {
		if d.Kind == codesignal.DiagKindProjectCoverageIncomplete || d.Kind == codesignal.DiagKindProjectLifecycleIndeterminate {
			return true
		}
	}
	return false
}
func renderProjectCoverageSection(b *strings.Builder, coverage *projectmodel.Coverage) {
	if coverage == nil {
		return
	}

	fmt.Fprintf(b, "\nProject coverage: phase=%s, complete=%t\n", coverage.Phase, coverage.Complete)
	writeSortedIntMap(b, "count", coverage.Counts)
	writeSortedIntMap(b, "budget", coverage.Budgets)
	for _, diagnostic := range coverage.Diagnostics {
		fmt.Fprintf(b, "  project diagnostic: %s: %s\n", diagnostic.Code, diagnostic.Message)
	}
}
func projectChangeSignalIDs(changes []codesignal.ProjectChange) map[string]struct{} {
	ids := make(map[string]struct{}, len(changes))
	for _, change := range changes {
		if change.ID != "" {
			ids[change.ID] = struct{}{}
		}
	}
	return ids
}
func writeOptionalLine(b *strings.Builder, label, value string) {
	if value == "" {
		return
	}
	fmt.Fprintf(b, "%s: %s\n", label, value)
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

// renderWithheldSignals writes the one line that says what a narrowed view
// left out. seeAllCommand is the invocation that shows everything.
func renderWithheldSignals(b *strings.Builder, withheld *codesignal.SignalsWithheld, seeAllCommand string) {
	if withheld == nil {
		return
	}
	var phrases []string
	var counts []int
	if withheld.MinSeverity != "" {
		phrases = append(phrases, "below --min-severity "+string(withheld.MinSeverity))
		counts = append(counts, withheld.BelowMinSeverity)
	}
	if withheld.Top > 0 {
		phrases = append(phrases, fmt.Sprintf("beyond --top %d", withheld.Top))
		counts = append(counts, withheld.BeyondTop)
	}
	if len(phrases) == 0 {
		return
	}

	total := 0
	for _, count := range counts {
		total += count
	}
	fmt.Fprintf(b, "withheld: %s %s", signalCountNoun(total), withheldBreakdown(phrases, counts))
	b.WriteString("; counts above describe the full analysis")
	if total > 0 && seeAllCommand != "" {
		fmt.Fprintf(b, "; see all: %s", seeAllCommand)
	}
	b.WriteString("\n")
}

func withheldBreakdown(phrases []string, counts []int) string {
	if len(phrases) == 1 {
		return phrases[0]
	}
	return fmt.Sprintf("(%d %s, %d %s)", counts[0], phrases[0], counts[1], phrases[1])
}

func signalCountNoun(n int) string {
	if n == 1 {
		return "1 signal"
	}
	return fmt.Sprintf("%d signals", n)
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
