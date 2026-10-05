package codesignalcli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

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

// RenderJSON renders report as its canonical JSON representation followed
// by exactly one trailing newline, with no CLI-only wrapper fields added.
func RenderJSON(report *codesignal.Report) ([]byte, error) {
	encoded, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}
