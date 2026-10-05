package codesignalcli

import (
	"fmt"

	"github.com/lousy-agents/coach/pkg/codesignal"

	"sort"
	"strings"
)

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
func renderOneProjectChange(b *strings.Builder, change codesignal.ProjectChange) {
	fmt.Fprintf(b, "semantic_key: %s\n", change.SemanticKey)
	fmt.Fprintf(b, "rule_id: %s\n", change.RuleID)
	fmt.Fprintf(b, "severity: %s\n", change.Severity)
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
func renderDiagnosticsSection(b *strings.Builder, diagnostics []codesignal.Diagnostic) {
	if len(diagnostics) == 0 {
		return
	}
	b.WriteString("\nDiagnostics:\n")
	for _, diagnostic := range diagnostics {
		renderDiagnostic(b, diagnostic)
	}
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
	renderWithheldSignals(b, report.SignalsWithheld)
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
