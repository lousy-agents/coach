package render

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func TestRenderTextDiagnosticsSection(t *testing.T) {
	report := &codesignal.Report{
		Diagnostics: []codesignal.Diagnostic{
			{Path: "a.go", Kind: "syntax_errors", Message: "unexpected token"},
			{Path: "", Kind: "not_a_git_worktree", Message: "no path available"},
		},
	}

	got := ReportText(report)

	for _, want := range []string{
		"a.go", "syntax_errors", "unexpected token",
		"not_a_git_worktree", "no path available",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered text missing %q; got:\n%s", want, got)
		}
	}
}
