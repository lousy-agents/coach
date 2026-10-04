package projectmodel_test

import (
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func countReachabilityDiagnostic(diags []projectmodel.Diagnostic, code string) int {
	n := 0
	for _, d := range diags {
		if d.Code == code {
			n++
		}
	}
	return n
}

func withoutWallClockCounts(cov projectmodel.Coverage) projectmodel.Coverage {
	counts := make(map[string]int, len(cov.Counts))
	for k, v := range cov.Counts {
		if k == "runtime_ms" || k == "memory_bytes" {
			continue
		}
		counts[k] = v
	}
	cov.Counts = counts
	return cov
}
