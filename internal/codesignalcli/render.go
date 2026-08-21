package codesignalcli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// RenderText renders report as deterministic, ANSI-free plain text.
func RenderText(report *codesignal.Report) string {
	var b strings.Builder
	renderReportSummary(&b, report)
	renderActiveFindings(&b, report)
	renderProjectFacts(&b, report.ProjectFacts)
	renderDiagnosticsSection(&b, report.Diagnostics)
	renderCoverageSection(&b, report.Coverage)
	renderProjectCoverageSection(&b, report.ProjectCoverage)
	return b.String()
}

func renderReportSummary(b *strings.Builder, report *codesignal.Report) {
	if report.Scope.Baseline {
		renderBaselineSummary(b, report)
		return
	}
	renderDiffSummary(b, report)
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

// renderNoActiveFindingsVerdict qualifies the verdict from a diagnostic Kind,
// not only report.ProjectCoverage.Complete: Report exposes only head-side
// coverage, and a diff's base side can be incomplete while the head side is
// complete, which the Complete field alone cannot see.
func renderNoActiveFindingsVerdict(b *strings.Builder, report *codesignal.Report) {
	incompleteProject := (report.ProjectCoverage != nil && !report.ProjectCoverage.Complete) ||
		hasProjectLifecycleDiagnostic(report.Diagnostics)
	hasDiagnostics := len(report.Diagnostics) > 0
	if !hasDiagnostics && !incompleteProject {
		b.WriteString("No active CodeSignal findings.\n")
		return
	}

	skippedPaths := distinctDiagnosticPaths(report.Diagnostics)
	var causes []string
	if n := len(skippedPaths); n > 0 {
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

func pathCountClause(n int) string {
	if n == 1 {
		return "1 path was not analyzed"
	}
	return fmt.Sprintf("%d paths were not analyzed", n)
}

func distinctDiagnosticPaths(diagnostics []codesignal.Diagnostic) map[string]struct{} {
	paths := make(map[string]struct{})
	for _, d := range diagnostics {
		if d.Path != "" {
			paths[d.Path] = struct{}{}
		}
	}
	return paths
}

func hasProjectLifecycleDiagnostic(diagnostics []codesignal.Diagnostic) bool {
	for _, d := range diagnostics {
		if d.Kind == codesignal.DiagKindProjectCoverageIncomplete || d.Kind == codesignal.DiagKindProjectLifecycleIndeterminate {
			return true
		}
	}
	return false
}

func renderFileLocalSignals(b *strings.Builder, signals []codesignal.Signal, projectSignalIDs map[string]struct{}) bool {
	rendered := false
	for _, signal := range signals {
		if _, isProject := projectSignalIDs[signal.ID]; isProject {
			continue
		}
		if rendered {
			b.WriteString("\n")
		}
		renderSignal(b, signal)
		rendered = true
	}
	return rendered
}

func renderDiagnosticsSection(b *strings.Builder, diagnostics []codesignal.Diagnostic) {
	if len(diagnostics) == 0 {
		return
	}
	b.WriteString("\nDiagnostics:\n")
	for _, diagnostic := range diagnostics {
		renderDiagnostic(b, diagnostic)
	}
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

func sumCoverageGroups(groups []codesignal.CoverageGroup) int {
	total := 0
	for _, g := range groups {
		total += g.Count
	}
	return total
}

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
		renderOneProjectChange(b, change)
		if i != len(report.ProjectChanges)-1 {
			b.WriteString("\n")
		}
	}
	renderProjectSummary(b, report.ProjectSummary)
}

func renderOneProjectChange(b *strings.Builder, change codesignal.ProjectChange) {
	fmt.Fprintf(b, "semantic_key: %s\n", change.SemanticKey)
	fmt.Fprintf(b, "rule_id: %s\n", change.RuleID)
	fmt.Fprintf(b, "path: %s\n", change.PrimaryAnchor.Path)
	fmt.Fprintf(b, "line: %d\n", change.PrimaryAnchor.Location.StartRow+1)
	fmt.Fprintf(b, "lifecycle: %s\n", change.Lifecycle)
	fmt.Fprintf(b, "changed: %t\n", change.Changed)
	writeOptionalLine(b, "evidence", change.Evidence)
	renderMachineEvidence(b, change.MachineEvidence)
	writeOptionalLine(b, "why it matters", change.WhyItMatters)
	writeOptionalLine(b, "recommendation", change.Recommendation)
	for _, location := range change.RelatedLocations {
		fmt.Fprintf(b, "related: %s:%d\n", location.Path, location.Location.StartRow+1)
	}
	for _, ref := range change.CoverageRefs {
		fmt.Fprintf(b, "coverage_ref: %s\n", ref)
	}
	renderPathSteps(b, change.PathSteps)
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

func renderProjectFacts(b *strings.Builder, facts []codesignal.ProjectFact) {
	if len(facts) == 0 {
		return
	}
	b.WriteString("\nFacts:\n")
	for i, fact := range facts {
		renderOneProjectFact(b, fact)
		if i != len(facts)-1 {
			b.WriteString("\n")
		}
	}
}

func renderOneProjectFact(b *strings.Builder, fact codesignal.ProjectFact) {
	fmt.Fprintf(b, "kind: %s\n", fact.Kind)
	writeOptionalLine(b, "semantic_key", fact.SemanticKey)
	writeOptionalLine(b, "evidence", fact.Evidence)
	for _, ref := range fact.CoverageRefs {
		fmt.Fprintf(b, "coverage_ref: %s\n", ref)
	}
	renderPathSteps(b, fact.PathSteps)
}

func writeOptionalLine(b *strings.Builder, label, value string) {
	if value == "" {
		return
	}
	fmt.Fprintf(b, "%s: %s\n", label, value)
}

func renderMachineEvidence(b *strings.Builder, evidence map[string]string) {
	if len(evidence) == 0 {
		return
	}
	keys := make([]string, 0, len(evidence))
	for key := range evidence {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(b, "machine_evidence.%s: %s\n", key, evidence[key])
	}
}

func renderPathSteps(b *strings.Builder, steps []codesignal.ProjectPathStep) {
	for _, step := range steps {
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

func writeSortedIntMap(b *strings.Builder, label string, values map[string]int) {
	if len(values) == 0 {
		return
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(b, "  %s: %s=%d\n", label, key, values[key])
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
