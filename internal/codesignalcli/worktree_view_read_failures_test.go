package codesignalcli

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// A --worktree scan that cannot read the Git state that decides what it can see
// (the sparse-checkout state, the gitlinks) must say so. If a failed read were
// treated as nothing to report, the scan would end in an unqualified all-clear
// while working-tree bytes may be hidden, and under --fail-on-incomplete-coverage
// it would exit 0. A failed status read, which fails OpenHead, is pinned in
// worktree_view_counts_test.go.
func TestWorktreeViewNamesAFailedGitReadInsteadOfGivingAnAllClear(t *testing.T) {
	tests := []struct {
		name  string
		deps  worktreeViewDeps
		cause string
	}{
		{
			"the sparse-checkout state cannot be read",
			worktreeViewDeps{sparseErr: errors.New("reading sparse state: index unreadable")},
			"reading sparse state: index unreadable",
		},
		{
			"the gitlinks cannot be read",
			worktreeViewDeps{gitlinksErr: errors.New("listing gitlinks: ls-files failed")},
			"listing gitlinks: ls-files failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := newWorktreeViewForTest(t, withEmptySnapshotAndDisk(tt.deps))

			diagnostics := view.Disclosure(".", "HEAD", "")

			if len(diagnostics) != 1 || diagnostics[0].Kind != codesignal.DiagKindWorktreeStatusCheckFailed {
				t.Fatalf("diagnostics = %+v, want exactly one %s", diagnostics, codesignal.DiagKindWorktreeStatusCheckFailed)
			}
			if message := diagnostics[0].Message; !strings.Contains(message, "working tree status check failed") || !strings.Contains(message, tt.cause) {
				t.Errorf("message %q must say the status check failed and carry the cause %q", message, tt.cause)
			}
		})
	}
}

func TestWorktreeViewReportsNoFailedReadWhenEveryGitReadSucceeds(t *testing.T) {
	view := newWorktreeViewForTest(t, withEmptySnapshotAndDisk(worktreeViewDeps{}))

	if got := countKind(view.Disclosure(".", "HEAD", ""), codesignal.DiagKindWorktreeStatusCheckFailed); got != 0 {
		t.Errorf("worktree_status_check_failed diagnostics = %d, want none when no read failed", got)
	}
}

// withEmptySnapshotAndDisk states the collaborators a failed-read case does not
// vary: a committed snapshot and a disk that hold nothing.
func withEmptySnapshotAndDisk(deps worktreeViewDeps) worktreeViewDeps {
	deps.committed = fstest.MapFS{}
	deps.disk = absentDisk{}
	return deps
}
