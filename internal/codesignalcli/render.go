package codesignalcli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// RenderText renders report as deterministic, ANSI-free plain text: a
// one-line summary, then either "No active CodeSignal findings." or one
// block per signal (in report.Signals order), then a diagnostics section
// when report.Diagnostics is non-empty.
func RenderText(report *codesignal.Report) string {
	var b strings.Builder

	fmt.Fprintf(&b, "files analyzed: %d, active signals: %d, diagnostics: %d\n",
		report.Summary.FilesAnalyzed, report.Summary.ActiveSignals, len(report.Diagnostics))

	if len(report.Signals) == 0 {
		b.WriteString("No active CodeSignal findings.\n")
	} else {
		for i, signal := range report.Signals {
			renderSignal(&b, signal)
			if i != len(report.Signals)-1 {
				b.WriteString("\n")
			}
		}
	}

	if len(report.Diagnostics) > 0 {
		b.WriteString("\nDiagnostics:\n")
		for _, diagnostic := range report.Diagnostics {
			renderDiagnostic(&b, diagnostic)
		}
	}

	return b.String()
}

func renderSignal(b *strings.Builder, signal codesignal.Signal) {
	fmt.Fprintf(b, "path: %s\n", signal.Path)
	fmt.Fprintf(b, "line: %d\n", signal.Location.StartRow+1)
	fmt.Fprintf(b, "lifecycle: %s\n", signal.Lifecycle)
	fmt.Fprintf(b, "changed: %t\n", signal.Changed)
	fmt.Fprintf(b, "evidence: %s\n", signal.Evidence)
	fmt.Fprintf(b, "why it matters: %s\n", signal.WhyItMatters)
	fmt.Fprintf(b, "recommendation: %s\n", signal.Recommendation)
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
