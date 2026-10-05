package render

import (
	"fmt"
	"sort"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

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
