package codesignalcli

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func TestWorktreeViewQualifiesTheVerdictForWhatGitWarnedAbout(t *testing.T) {
	tests := []struct {
		name     string
		warnings gitrepo.StatusWarnings
		want     []string
	}{
		{"a directory git could not open", gitrepo.StatusWarnings{UnreadableDirectories: []string{"pkg/y/"}}, []string{"git could not open pkg/y/", "make it readable"}},
		{"a directory whose name is escaped", gitrepo.StatusWarnings{UnreadableDirectories: []string{`"tab\there/"`}}, []string{`git could not open "tab\there/"`}},
		{"a warning coach does not recognize", gitrepo.StatusWarnings{Unrecognized: true}, []string{"does not recognize", "may not have been read"}},
		{"both", gitrepo.StatusWarnings{UnreadableDirectories: []string{"a/"}, Unrecognized: true}, []string{"git could not open a/", "does not recognize"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertScanIncompleteCause(t, tt.warnings, tt.want)
		})
	}
}

func assertScanIncompleteCause(t *testing.T, warnings gitrepo.StatusWarnings, want []string) {
	t.Helper()
	view := newWorktreeViewForTest(t, worktreeViewDeps{
		status:    WorktreeStatus{Warnings: warnings},
		committed: fstest.MapFS{},
		disk:      absentDisk{},
	})

	var messages []string
	for _, diagnostic := range view.Disclosure(".", "HEAD", "") {
		if diagnostic.Kind == codesignal.DiagKindWorktreeScanIncomplete {
			messages = append(messages, diagnostic.Message)
		}
	}

	if len(messages) != 1 {
		t.Fatalf("worktree_scan_incomplete diagnostics = %v, want exactly one", messages)
	}
	for _, fragment := range want {
		if !strings.Contains(messages[0], fragment) {
			t.Errorf("message %q does not contain %q", messages[0], fragment)
		}
	}
}

func TestWorktreeViewNamesNoCauseWhenGitWarnedAboutNothing(t *testing.T) {
	view := newWorktreeViewForTest(t, worktreeViewDeps{status: WorktreeStatus{}, committed: fstest.MapFS{}, disk: absentDisk{}})

	if got := countKind(view.Disclosure(".", "HEAD", ""), codesignal.DiagKindWorktreeScanIncomplete); got != 0 {
		t.Errorf("worktree_scan_incomplete diagnostics = %d, want none", got)
	}
}
