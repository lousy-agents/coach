package codesignalcli

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// A git that cannot be told not to fetch is given the status that does not look into submodules, so an edit inside a
// submodule is not in it. A scan that said nothing would report a tree with such an edit as clean.
func TestWorktreeViewQualifiesTheVerdictWhenTheStatusSkippedTheSubmodulesItHas(t *testing.T) {
	view := newWorktreeViewForTest(t, worktreeViewDeps{
		status:    WorktreeStatus{SubmodulesSkippedBy: "2.43.0"},
		committed: fstest.MapFS{},
		disk:      absentDisk{},
		gitlinks:  []string{"vendored"},
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
	for _, fragment := range []string{"git 2.43.0", "vendored", "submodule"} {
		if !strings.Contains(messages[0], fragment) {
			t.Errorf("message %q does not contain %q", messages[0], fragment)
		}
	}
}

func TestWorktreeViewNamesNoSkippedSubmodulesWhenThereIsNoneToSkipOrNoneWasSkipped(t *testing.T) {
	assertNoScanIncomplete(t, WorktreeStatus{SubmodulesSkippedBy: "2.43.0"}, nil)
	assertNoScanIncomplete(t, WorktreeStatus{}, []string{"vendored"})
}

func assertNoScanIncomplete(t *testing.T, status WorktreeStatus, gitlinks []string) {
	t.Helper()
	view := newWorktreeViewForTest(t, worktreeViewDeps{status: status, committed: fstest.MapFS{}, disk: absentDisk{}, gitlinks: gitlinks})

	if got := countKind(view.Disclosure(".", "HEAD", ""), codesignal.DiagKindWorktreeScanIncomplete); got != 0 {
		t.Errorf("status %+v with gitlinks %v: worktree_scan_incomplete diagnostics = %d, want none", status, gitlinks, got)
	}
}
