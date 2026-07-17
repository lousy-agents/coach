package codesignalcli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// RenderText renders report as deterministic, ANSI-free plain text: a
// one-line summary, then either "No active CodeSignal findings." or one
// block per signal (in report.Signals order), then a diagnostics section
// when report.Diagnostics is non-empty, then a Coverage section summarizing
// unsupported/excluded files by reason and language rather than one line per
// file (written only when report.Coverage has groups to show, for both
// baseline and non-baseline reports). A Repository Baseline report
// (report.Scope.Baseline) renders a distinct summary line that identifies
// the analyzed revision and states plainly that the result is not a diff
// comparison; everything else (signal blocks, diagnostics section) is
// unchanged.
func RenderText(report *codesignal.Report) string {
	var b strings.Builder

	if report.Scope.Baseline {
		renderBaselineSummary(&b, report)
	} else {
		renderDiffSummary(&b, report)
	}

	if len(report.Signals) == 0 {
		b.WriteString("No active CodeSignal findings.\n")
	} else {
		for i, signal := range report.Signals {
			renderSignal(&b, signal)
			if i != len(report.Signals)-1 {
				b.WriteString("\n")
			}
		}
	}

	if len(report.Diagnostics) > 0 {
		b.WriteString("\nDiagnostics:\n")
		for _, diagnostic := range report.Diagnostics {
			renderDiagnostic(&b, diagnostic)
		}
	}

	renderCoverageSection(&b, report.Coverage)

	return b.String()
}

// renderBaselineSummary writes the Repository Baseline summary line: the
// analyzed revision, an explicit statement that this is not a diff
// comparison, and file-discovery/coverage counts. report.Coverage is
// nil-checked defensively -- a nil Coverage falls back to treating every
// count as 0 rather than panicking.
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

// renderDiffSummary writes the non-baseline (base-diff) summary line. When
// report.Scope.AppliedScope was actually populated by the diff flow ("all" or
// "production"), it prepends a scope clause disclosing the applied scope and,
// for "production", the number of files filtered out by that scope
// (report.Coverage.Excluded, nil-safe) -- distinguishing "scope: production,
// filtered: 0" (scoped, nothing happened to match) from "all" (no scope
// filtering applied at all). When AppliedScope is empty (not populated by the
// diff flow, e.g. an older/unrelated caller), the line is left in its
// original format with no scope clause.
func renderDiffSummary(b *strings.Builder, report *codesignal.Report) {
	switch report.Scope.AppliedScope {
	case "all":
		fmt.Fprintf(b, "scope: all (no scope filtering applied), ")
	case "":
		// No scope clause: AppliedScope was never populated.
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

// sumCoverageGroups totals CoverageGroup.Count across groups so the
// top-line summary can report a single count per bucket without printing
// one line per group there.
func sumCoverageGroups(groups []codesignal.CoverageGroup) int {
	total := 0
	for _, g := range groups {
		total += g.Count
	}
	return total
}

// renderCoverageSection writes one line per CoverageGroup in
// coverage.Unsupported and coverage.Excluded, staying proportional to the
// number of distinct reason/language combinations rather than the number of
// files. Writes nothing when coverage is nil or has no groups.
func renderCoverageSection(b *strings.Builder, coverage *codesignal.Coverage) {
	if coverage == nil || (len(coverage.Unsupported) == 0 && len(coverage.Excluded) == 0) {
		return
	}

	b.WriteString("\nCoverage:\n")
	for _, g := range coverage.Unsupported {
		fmt.Fprintf(b, "  unsupported: %d %s files\n", g.Count, g.Language)
	}
	for _, g := range coverage.Excluded {
		fmt.Fprintf(b, "  excluded: %d %s %s files\n", g.Count, g.Reason, g.Language)
	}
}

func renderSignal(b *strings.Builder, signal codesignal.Signal) {
	fmt.Fprintf(b, "path: %s\n", signal.Path)
	fmt.Fprintf(b, "line: %d\n", signal.Location.StartRow+1)
	fmt.Fprintf(b, "lifecycle: %s\n", signal.Lifecycle)
	fmt.Fprintf(b, "source_scope: %s\n", signal.SourceScope)
	fmt.Fprintf(b, "changed: %t\n", signal.Changed)
	fmt.Fprintf(b, "evidence: %s\n", signal.Evidence)
	fmt.Fprintf(b, "why it matters: %s\n", signal.WhyItMatters)
	fmt.Fprintf(b, "recommendation: %s\n", signal.Recommendation)
}

func renderDiagnostic(b *strings.Builder, diagnostic codesignal.Diagnostic) {
	fmt.Fprintf(b, "path: %s, kind: %s, message: %s\n", diagnostic.Path, diagnostic.Kind, diagnostic.Message)
}

// RenderJSON renders report as its canonical JSON representation followed
// by exactly one trailing newline, with no CLI-only wrapper fields added.
func RenderJSON(report *codesignal.Report) ([]byte, error) {
	encoded, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}
