package render

import (
	"strconv"
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

func TestRenderTextDiagnosticRootShownQuotedWhenItHoldsControlOrInvalidBytes(t *testing.T) {
	tests := []struct {
		name string
		root string
		want bool
	}{
		{"a plain root is shown as is", "lib", false},
		{"an escape sequence is quoted", "ctl\x1b[31mred", true},
		{"a newline is quoted", "sp ace/in\nner", true},
		{"invalid UTF-8 is quoted", "bad\xffname", true},
	}
	for _, tt := range tests {
		report := &codesignal.Report{Diagnostics: []codesignal.Diagnostic{{
			Kind: codesignal.DiagKindWorktreeScanIncomplete, Reason: "skip_worktree", Root: tt.root,
			HidesInScope: codesignal.HidesInScopeNo, Message: "skip-worktree keeps tracked paths out of git status",
		}}}

		got := ReportText(report)

		if !tt.want {
			if !strings.Contains(got, "root: "+tt.root+"/") {
				t.Errorf("%s: want root shown as is; got:\n%s", tt.name, got)
			}
			continue
		}
		outside := "root: " + strconv.Quote(tt.root) + "/"
		inside := "root: " + strconv.Quote(tt.root+"/")
		if !strings.Contains(got, outside) && !strings.Contains(got, inside) {
			t.Errorf("%s: want root %q quoted as %s or %s; got:\n%s", tt.name, tt.root, outside, inside, got)
		}
		if strings.Contains(got, tt.root) {
			t.Errorf("%s: the raw root reached the text output:\n%q", tt.name, got)
		}
	}
}
