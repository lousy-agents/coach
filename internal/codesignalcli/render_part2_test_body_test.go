package codesignalcli

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func body_renderPart2Test_94(t *testing.T, tt struct {
	name     string
	startRow uint
	wantLine string
}) {
	report := &codesignal.Report{
		Signals: []codesignal.Signal{{Path: "a.go", Location: semantics.Location{StartRow: tt.startRow}}},
	}

	got := RenderText(report)

	if !strings.Contains(got, tt.wantLine) {
		t.Errorf("rendered text missing %q; got:\n%s", tt.wantLine, got)
	}
}
