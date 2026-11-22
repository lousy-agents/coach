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
		want     []codesignal.Diagnostic
	}{
		{"a directory git could not open", gitrepo.StatusWarnings{UnreadableDirectories: []string{"pkg/y"}}, []codesignal.Diagnostic{{Reason: "unreadable_directory", Root: "pkg/y"}}},
		{"a directory whose name is escaped", gitrepo.StatusWarnings{UnreadableDirectories: []string{`"tab\there"`}}, []codesignal.Diagnostic{{Reason: "unreadable_directory", Root: `"tab\there"`}}},
		{"a warning coach does not recognize", gitrepo.StatusWarnings{Unrecognized: 1}, []codesignal.Diagnostic{{Reason: "unrecognized_git_warning"}}},
		{"both", gitrepo.StatusWarnings{UnreadableDirectories: []string{"a"}, Unrecognized: 1}, []codesignal.Diagnostic{{Reason: "unreadable_directory", Root: "a"}, {Reason: "unrecognized_git_warning"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertScanIncompleteEntries(t, tt.warnings, tt.want)
		})
	}
}

func assertScanIncompleteEntries(t *testing.T, warnings gitrepo.StatusWarnings, want []codesignal.Diagnostic) {
	t.Helper()
	view := newWorktreeViewForTest(t, worktreeViewDeps{
		status:    WorktreeStatus{Warnings: warnings},
		committed: fstest.MapFS{},
		disk:      absentDisk{},
	})

	got := scanIncompleteEntries(view.Disclosure(".", "HEAD", ""))

	if len(got) != len(want) {
		t.Fatalf("worktree_scan_incomplete diagnostics = %+v, want %d", got, len(want))
	}
	for i, entry := range got {
		if entry.Reason != want[i].Reason || entry.Root != want[i].Root || entry.HidesInScope != codesignal.HidesInScopeUnknown || entry.Path != "" {
			t.Errorf("entry %d = %+v, want reason %q at root %q, unknown, with no path", i, entry, want[i].Reason, want[i].Root)
		}
		if strings.Contains(entry.Message, "warning:") || strings.Contains(entry.Message, entry.Root+"/") && entry.Root != "" {
			t.Errorf("message %q relays git's words or the directory", entry.Message)
		}
	}
}

func TestWorktreeViewNamesNoCauseWhenGitWarnedAboutNothing(t *testing.T) {
	view := newWorktreeViewForTest(t, worktreeViewDeps{status: WorktreeStatus{}, committed: fstest.MapFS{}, disk: absentDisk{}})

	if got := countKind(view.Disclosure(".", "HEAD", ""), codesignal.DiagKindWorktreeScanIncomplete); got != 0 {
		t.Errorf("worktree_scan_incomplete diagnostics = %d, want none", got)
	}
}
