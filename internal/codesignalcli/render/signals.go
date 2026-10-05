package render

import (
	"fmt"
	"sort"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

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
