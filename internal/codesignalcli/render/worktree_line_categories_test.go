package render

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func TestWorktreeLineReportsATreeWithOnlyOneKindOfChangeAsChanged(t *testing.T) {
	t.Run("only modified files", func(t *testing.T) {
		assertWorktreeLine(t, codesignal.WorktreeScope{Modified: 1}, "worktree: 1 modified, 0 added, 0 deleted, 0 untracked on revision 0123456789ab.\n")
	})
	t.Run("only added files", func(t *testing.T) {
		assertWorktreeLine(t, codesignal.WorktreeScope{Added: 1}, "worktree: 0 modified, 1 added, 0 deleted, 0 untracked on revision 0123456789ab.\n")
	})
	t.Run("only deleted files", func(t *testing.T) {
		assertWorktreeLine(t, codesignal.WorktreeScope{Deleted: 1}, "worktree: 0 modified, 0 added, 1 deleted, 0 untracked on revision 0123456789ab.\n")
	})
	t.Run("only untracked files", func(t *testing.T) {
		assertWorktreeLine(t, codesignal.WorktreeScope{Untracked: 1}, "worktree: 0 modified, 0 added, 0 deleted, 1 untracked on revision 0123456789ab.\n")
	})
}

func assertWorktreeLine(t *testing.T, counts codesignal.WorktreeScope, want string) {
	t.Helper()
	if got := worktreeLineOf(t, worktreeReport(counts)); got != want {
		t.Errorf("rendered %q, want %q: a tree with uncommitted changes is never reported as clean", got, want)
	}
}
