package gitrepo

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// A submodule is one entry whose path git also matches through a trailing slash, so the listing of what is
// below it returns the submodule itself. That is the entry of the path, not another path's.
func TestIndexEntriesReturnsASubmodulesOwnEntry(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	gitfixture.CommitGitlink(t, repo, "vendor/sub")

	got, err := IndexEntries(RunBytes, repo, "vendor/sub")

	if err != nil || len(got.Entries) != 1 || got.Entries[0].Mode != "160000" || got.HoldsBelow {
		t.Errorf("IndexEntries = %+v, %v; want the one 160000 entry of the submodule", got, err)
	}
}
