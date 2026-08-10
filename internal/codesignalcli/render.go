package codesignalcli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// RenderText renders report as deterministic, ANSI-free plain text: a
// one-line summary, then either "No active CodeSignal findings." or blocks
// per file-local signal and project observation (in report order), then a
// diagnostics section when report.Diagnostics is non-empty, then file and
// project coverage sections. A Repository Baseline report
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

	// Build projects anchored ProjectChanges onto Signals for JSON consumers
	// while retaining project_changes for structured fields. Text must present
	// each logical observation once: file-local signals via the signal block,
	// project-origin findings only under Project findings (matched by Signal.ID).
	projectSignalIDs := projectChangeSignalIDs(report.ProjectChanges)

	if len(report.Signals) == 0 && len(report.ProjectChanges) == 0 {
		b.WriteString("No active CodeSignal findings.\n")
		renderProjectSummary(&b, report.ProjectSummary)
	} else {
		renderedSignal := false
		for _, signal := range report.Signals {
			if _, isProject := projectSignalIDs[signal.ID]; isProject {
				continue
			}
			if renderedSignal {
				b.WriteString("\n")
			}
			renderSignal(&b, signal)
			renderedSignal = true
		}
		if len(report.ProjectChanges) > 0 {
			if renderedSignal {
				b.WriteString("\n")
			}
			renderProjectChanges(&b, report)
		} else if report.ProjectSummary != nil {
			renderProjectSummary(&b, report.ProjectSummary)
		}
	}

	renderProjectFacts(&b, report.ProjectFacts)

	if len(report.Diagnostics) > 0 {
		b.WriteString("\nDiagnostics:\n")
		for _, diagnostic := range report.Diagnostics {
			renderDiagnostic(&b, diagnostic)
		}
	}

	renderCoverageSection(&b, report.Coverage)
	renderProjectCoverageSection(&b, report.ProjectCoverage)

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

// projectChangeSignalIDs returns the set of Signal IDs Build assigns to
// project observations so text rendering can skip the plain signal body for
// those entries and emit the structured Project findings block once.
func projectChangeSignalIDs(changes []codesignal.ProjectChange) map[string]struct{} {
	ids := make(map[string]struct{}, len(changes))
	for _, change := range changes {
		if change.ID != "" {
			ids[change.ID] = struct{}{}
		}
	}
	return ids
}

func renderProjectChanges(b *strings.Builder, report *codesignal.Report) {
	b.WriteString("Project findings:\n")
	for i, change := range report.ProjectChanges {
		fmt.Fprintf(b, "semantic_key: %s\n", change.SemanticKey)
		fmt.Fprintf(b, "rule_id: %s\n", change.RuleID)
		fmt.Fprintf(b, "path: %s\n", change.PrimaryAnchor.Path)
		fmt.Fprintf(b, "line: %d\n", change.PrimaryAnchor.Location.StartRow+1)
		fmt.Fprintf(b, "lifecycle: %s\n", change.Lifecycle)
		fmt.Fprintf(b, "changed: %t\n", change.Changed)
		if change.Evidence != "" {
			fmt.Fprintf(b, "evidence: %s\n", change.Evidence)
		}
		if len(change.MachineEvidence) > 0 {
			keys := make([]string, 0, len(change.MachineEvidence))
			for key := range change.MachineEvidence {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				fmt.Fprintf(b, "machine_evidence.%s: %s\n", key, change.MachineEvidence[key])
			}
		}
		if change.WhyItMatters != "" {
			fmt.Fprintf(b, "why it matters: %s\n", change.WhyItMatters)
		}
		if change.Recommendation != "" {
			fmt.Fprintf(b, "recommendation: %s\n", change.Recommendation)
		}
		for _, location := range change.RelatedLocations {
			fmt.Fprintf(b, "related: %s:%d\n", location.Path, location.Location.StartRow+1)
		}
		for _, ref := range change.CoverageRefs {
			fmt.Fprintf(b, "coverage_ref: %s\n", ref)
		}
		for _, step := range change.PathSteps {
			fmt.Fprintf(b, "path step: %s", step.NodeID)
			if step.DisplayName != "" {
				fmt.Fprintf(b, " (%s)", step.DisplayName)
			}
			if step.Resolution != "" {
				fmt.Fprintf(b, ", resolution: %s", step.Resolution)
			}
			if step.Confidence != "" {
				fmt.Fprintf(b, ", confidence: %s", step.Confidence)
			}
			b.WriteByte('\n')
			for _, location := range step.SourceLocations {
				fmt.Fprintf(b, "  source: %s:%d\n", location.Path, location.Location.StartRow+1)
			}
		}
		if i != len(report.ProjectChanges)-1 {
			b.WriteString("\n")
		}
	}
	renderProjectSummary(b, report.ProjectSummary)
}

func renderProjectSummary(b *strings.Builder, summary *codesignal.ProjectSummary) {
	if summary == nil {
		return
	}
	fmt.Fprintf(b, "Project summary: active=%d, introduced=%d, existing=%d, resolved=%d, baseline=%d\n",
		summary.ActiveChanges,
		summary.IntroducedChanges,
		summary.ExistingChanges,
		summary.ResolvedChanges,
		summary.BaselineChanges)
}

// renderProjectFacts writes the deterministic Facts section for facts-only
// observations. Facts never appear under Project findings or active counters.
func renderProjectFacts(b *strings.Builder, facts []codesignal.ProjectFact) {
	if len(facts) == 0 {
		return
	}
	b.WriteString("\nFacts:\n")
	for i, fact := range facts {
		fmt.Fprintf(b, "kind: %s\n", fact.Kind)
		if fact.SemanticKey != "" {
			fmt.Fprintf(b, "semantic_key: %s\n", fact.SemanticKey)
		}
		if fact.Evidence != "" {
			fmt.Fprintf(b, "evidence: %s\n", fact.Evidence)
		}
		for _, ref := range fact.CoverageRefs {
			fmt.Fprintf(b, "coverage_ref: %s\n", ref)
		}
		for _, step := range fact.PathSteps {
			fmt.Fprintf(b, "path step: %s", step.NodeID)
			if step.DisplayName != "" {
				fmt.Fprintf(b, " (%s)", step.DisplayName)
			}
			if step.Resolution != "" {
				fmt.Fprintf(b, ", resolution: %s", step.Resolution)
			}
			if step.Confidence != "" {
				fmt.Fprintf(b, ", confidence: %s", step.Confidence)
			}
			b.WriteByte('\n')
			for _, location := range step.SourceLocations {
				fmt.Fprintf(b, "  source: %s:%d\n", location.Path, location.Location.StartRow+1)
			}
		}
		if i != len(facts)-1 {
			b.WriteString("\n")
		}
	}
}

func renderProjectCoverageSection(b *strings.Builder, coverage *projectmodel.Coverage) {
	if coverage == nil {
		return
	}

	fmt.Fprintf(b, "\nProject coverage: phase=%s, complete=%t\n", coverage.Phase, coverage.Complete)
	keys := make([]string, 0, len(coverage.Counts))
	for key := range coverage.Counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(b, "  count: %s=%d\n", key, coverage.Counts[key])
	}
	keys = keys[:0]
	for key := range coverage.Budgets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(b, "  budget: %s=%d\n", key, coverage.Budgets[key])
	}
	for _, diagnostic := range coverage.Diagnostics {
		fmt.Fprintf(b, "  project diagnostic: %s: %s\n", diagnostic.Code, diagnostic.Message)
	}
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
