package codesignalcli

import (
	"fmt"

	"github.com/lousy-agents/coach/pkg/codesignal"

	"sort"
	"strings"
)

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
func renderReportSummary(b *strings.Builder, report *codesignal.Report) {
	if report.Scope.Baseline {
		renderBaselineSummary(b, report)
		return
	}
	renderDiffSummary(b, report)
}
