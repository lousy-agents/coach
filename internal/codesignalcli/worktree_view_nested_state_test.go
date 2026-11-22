package codesignalcli

import (
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

func TestUntrackedRepositoriesNeedAnUntrackedEntryThatNamesADirectory(t *testing.T) {
	t.Run("an untracked directory", func(t *testing.T) {
		assertUntrackedRepositories(t, []gitrepo.WorktreeEntry{{Code: "??", Path: "vendor/nested/"}}, []string{"vendor/nested"})
	})
	t.Run("an untracked file", func(t *testing.T) {
		assertUntrackedRepositories(t, []gitrepo.WorktreeEntry{{Code: "??", Path: "vendor/nested.go"}}, nil)
	})
	t.Run("a tracked entry that names a directory", func(t *testing.T) {
		assertUntrackedRepositories(t, []gitrepo.WorktreeEntry{{Code: " M", Path: "vendor/nested/"}}, nil)
	})
	t.Run("nothing changed", func(t *testing.T) { assertUntrackedRepositories(t, nil, nil) })
}

func assertUntrackedRepositories(t *testing.T, entries []gitrepo.WorktreeEntry, want []string) {
	t.Helper()
	if got := untrackedRepositories(entries); !reflect.DeepEqual(got, want) {
		t.Errorf("untrackedRepositories(%+v) = %v, want %v", entries, got, want)
	}
}

func TestDirtySubmodulesLeavesOutOnesWhoseDirectoryIsGone(t *testing.T) {
	entries := []gitrepo.WorktreeEntry{{Code: " M", Path: "third/lib"}, {Code: " D", Path: "third/gone"}, {Code: " M", Path: "pkg/a.go"}}
	submodules := map[string]bool{"third/lib": true, "third/gone": true}

	if got, want := dirtySubmodules(entries, submodules), []string{"third/lib"}; !reflect.DeepEqual(got, want) {
		t.Errorf("dirtySubmodules = %v, want %v", got, want)
	}
}
