package codesignalcli

import (
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

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
