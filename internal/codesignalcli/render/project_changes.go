package render

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

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

func renderOneProjectFact(b *strings.Builder, fact codesignal.ProjectFact) {
	fmt.Fprintf(b, "kind: %s\n", fact.Kind)
	writeOptionalLine(b, "semantic_key", fact.SemanticKey)
	writeOptionalLine(b, "evidence", fact.Evidence)
	for _, ref := range fact.CoverageRefs {
		fmt.Fprintf(b, "coverage_ref: %s\n", ref)
	}
	renderPathSteps(b, fact.PathSteps)
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
