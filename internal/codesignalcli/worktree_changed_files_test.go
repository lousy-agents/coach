package codesignalcli

import (
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

func TestAPathStagedThenDeletedFromDiskSelectsNothing(t *testing.T) {
	entries := []gitrepo.WorktreeEntry{
		{Code: "RD", Path: "new.go", OldPath: "old.go"},
		{Code: "AD", Path: "fresh.go"},
		{Code: "CD", Path: "copy.go", OldPath: "src.go"},
	}

	if got := changedFilesFromStatus(entries, nil); len(got) != 0 {
		t.Errorf("changedFilesFromStatus selected %+v, want nothing: no vanished new path has bytes to analyze", got)
	}
	if got := stagedRenames(entries); len(got) != 0 {
		t.Errorf("stagedRenames = %+v, want none: a vanished new path is not a rename to select", got)
	}
}

func TestAnIntentToAddRenameIsSelectedLikeAStagedOne(t *testing.T) {
	entries := []gitrepo.WorktreeEntry{{Code: " R", Path: "new.go", OldPath: "old.go"}}

	if got := changedFilesFromStatus(entries, nil); len(got) != 1 || got[0].Path != "new.go" || got[0].Status != "added" {
		t.Errorf("changedFilesFromStatus = %+v, want new.go as an added file: the move put its bytes on disk", got)
	}
	if got := stagedRenames(entries); !reflect.DeepEqual(got, entries) {
		t.Errorf("stagedRenames = %+v, want the intent-to-add rename, so its continuity is recorded", got)
	}
}

func TestARenameSourceWrittenAgainAsAnUntrackedFileIsSelectedAsAModification(t *testing.T) {
	entries := []gitrepo.WorktreeEntry{
		{Code: "R ", Path: "b.go", OldPath: "m.go"},
		{Code: "??", Path: "m.go"},
		{Code: "??", Path: "fresh.go"},
	}

	statuses := map[string]string{}
	for _, file := range changedFilesFromStatus(entries, nil) {
		statuses[file.Path] = string(file.Status)
	}

	want := map[string]string{"b.go": "added", "m.go": "modified", "fresh.go": "added"}
	if !reflect.DeepEqual(statuses, want) {
		t.Errorf("statuses = %v, want %v: HEAD holds m.go as the rename's source, so writing it again edits it", statuses, want)
	}
}
