package render

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func TestRenderTextSummaryLine(t *testing.T) {
	report := &codesignal.Report{
		Summary:     codesignal.Summary{FilesAnalyzed: 3, ActiveSignals: 2},
		Diagnostics: []codesignal.Diagnostic{{Path: "a.go", Kind: "k", Message: "m"}},
	}

	got := ReportText(report)

	for _, want := range []string{"files analyzed", "active signals", "diagnostics"} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered text missing summary substring %q; got:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "files analyzed: 3") {
		t.Errorf("expected files analyzed count 3; got:\n%s", got)
	}
	if !strings.Contains(got, "active signals: 2") {
		t.Errorf("expected active signals count 2; got:\n%s", got)
	}
	if !strings.Contains(got, "diagnostics: 1") {
		t.Errorf("expected diagnostics count 1; got:\n%s", got)
	}
}
