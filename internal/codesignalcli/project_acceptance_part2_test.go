package codesignalcli

import (
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func countDiagnosticsOfKind(diagnostics []codesignal.Diagnostic, kind string) int {
	count := 0
	for _, d := range diagnostics {
		if d.Kind == kind {
			count++
		}
	}
	return count
}
