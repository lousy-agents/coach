package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestACommittedRemovalTheWorkingTreeWroteAgainIsAnEditAgainstTheMergeBase(t *testing.T) {
	writtenAs := map[string]gitrepo.WorktreeEntry{
		"an untracked file":           {Code: "??", Path: "m.go"},
		"a staged addition":           {Code: "A ", Path: "m.go"},
		"the destination of a rename": {Code: "R ", Path: "m.go", OldPath: "n.go"},
	}
	for name, entry := range writtenAs {
		assertRecreatedFileIsModified(t, name, entry)
	}
}

func assertRecreatedFileIsModified(t *testing.T, name string, entry gitrepo.WorktreeEntry) {
	t.Helper()
	removed := gitrepo.SelectedFile{Path: "m.go", Status: "removed", Language: semantics.LanguageGo}
	entries := []gitrepo.WorktreeEntry{entry}

	got := statusInWorkingTree(removed, deletedTrackedPaths(entries), writtenPaths(entries))

	if got != codesignal.ChangeStatus("modified") {
		t.Errorf("%s: status = %q, want modified", name, got)
	}
}

func TestACommittedRemovalNothingWroteAgainStaysRemoved(t *testing.T) {
	removed := gitrepo.SelectedFile{Path: "m.go", Status: "removed", Language: semantics.LanguageGo}

	if got := statusInWorkingTree(removed, nil, nil); got != "removed" {
		t.Errorf("status with nothing written = %q, want removed", got)
	}
}
