package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

func TestHasUntrackedRepositoryNeedsAnUntrackedEntryThatNamesADirectory(t *testing.T) {
	t.Run("an untracked directory", func(t *testing.T) {
		assertUntrackedRepository(t, []gitrepo.WorktreeEntry{{Code: "??", Path: "vendor/nested/"}}, true)
	})
	t.Run("an untracked file", func(t *testing.T) {
		assertUntrackedRepository(t, []gitrepo.WorktreeEntry{{Code: "??", Path: "vendor/nested.go"}}, false)
	})
	t.Run("a tracked entry that names a directory", func(t *testing.T) {
		assertUntrackedRepository(t, []gitrepo.WorktreeEntry{{Code: " M", Path: "vendor/nested/"}}, false)
	})
	t.Run("nothing changed", func(t *testing.T) { assertUntrackedRepository(t, nil, false) })
}

func assertUntrackedRepository(t *testing.T, entries []gitrepo.WorktreeEntry, want bool) {
	t.Helper()
	if got := hasUntrackedRepository(entries); got != want {
		t.Errorf("hasUntrackedRepository(%+v) = %v, want %v", entries, got, want)
	}
}
