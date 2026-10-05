package codesignalcli

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func body_renderPart4Test_155(t *testing.T, tt struct {
	name         string
	report       *codesignal.Report
	wantContains []string
	wantAbsent   []string
}) {
	got := RenderText(tt.report)
	for _, want := range tt.wantContains {
		if !strings.Contains(got, want) {
			t.Errorf("rendered text missing %q; got:\n%s", want, got)
		}
	}
	for _, absent := range tt.wantAbsent {
		if strings.Contains(got, absent) {
			t.Errorf("rendered text must not contain %q; got:\n%s", absent, got)
		}
	}
}
