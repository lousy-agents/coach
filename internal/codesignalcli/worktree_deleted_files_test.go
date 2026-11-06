package codesignalcli

import (
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestARenameWhoseNewPathWasDeletedFromDiskDeletesItsSource(t *testing.T) {
	entries := []gitrepo.WorktreeEntry{
		{Code: "RD", Path: "new.go", OldPath: "old.go"},
		{Code: "CD", Path: "copy.go", OldPath: "src.go"},
	}

	wantTracked := map[string]semantics.Language{"old.go": semantics.LanguageGo}
	if got := deletedTrackedPaths(entries); !reflect.DeepEqual(got, wantTracked) {
		t.Errorf("deletedTrackedPaths = %v, want %v: the source of the rename is gone, the source of a copy stays", got, wantTracked)
	}
	wantAny := map[string]bool{"old.go": true}
	if got := deletedPaths(entries); !reflect.DeepEqual(got, wantAny) {
		t.Errorf("deletedPaths = %v, want %v", got, wantAny)
	}
}

func TestAnIntentToAddRenameTakesItsSourceOutOfTheBaselineOnly(t *testing.T) {
	entries := []gitrepo.WorktreeEntry{{Code: " R", Path: "new.go", OldPath: "old.go"}}

	if got := deletedPaths(entries); !reflect.DeepEqual(got, map[string]bool{"old.go": true}) {
		t.Errorf("deletedPaths = %v, want old.go: the baseline counts the destination as new, so the source must leave", got)
	}
	if got := deletedTrackedPaths(entries); len(got) != 0 {
		t.Errorf("deletedTrackedPaths = %v, want none: a diff scan leaves a moved file's old path to the continuity diagnostic, as for a staged rename", got)
	}
}

func TestAStagedRenameTakesItsSourceOutOfTheBaseline(t *testing.T) {
	entries := []gitrepo.WorktreeEntry{
		{Code: "R ", Path: "new.sh", OldPath: "old.sh"},
		{Code: "RM", Path: "edited.go", OldPath: "before.go"},
		{Code: "C ", Path: "copy.go", OldPath: "src.go"},
	}

	want := map[string]bool{"old.sh": true, "before.go": true}
	if got := deletedPaths(entries); !reflect.DeepEqual(got, want) {
		t.Errorf("deletedPaths = %v, want %v: a rename's source is gone, a copy's stays", got, want)
	}
}
