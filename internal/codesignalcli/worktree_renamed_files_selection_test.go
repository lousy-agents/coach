package codesignalcli

import (
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func TestWithoutRenamedPathsLeavesItsInputIntact(t *testing.T) {
	files := []gitrepo.SelectedFile{{Path: "a.go"}, {Path: "b.go"}, {Path: "c.go"}}
	renames := []gitrepo.WorktreeEntry{{Code: "R ", Path: "a.go", OldPath: "old.go"}}

	got := withoutRenamedPaths(files, renames)

	if want := []gitrepo.SelectedFile{{Path: "b.go"}, {Path: "c.go"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("withoutRenamedPaths = %+v, want %+v", got, want)
	}
	if want := []gitrepo.SelectedFile{{Path: "a.go"}, {Path: "b.go"}, {Path: "c.go"}}; !reflect.DeepEqual(files, want) {
		t.Errorf("the caller's slice is now %+v, want it unchanged: filtering must build a new slice", files)
	}
}

func TestSelectStagedRenamesDoesNotSelectAgainAPathTheCommittedDiffSelected(t *testing.T) {
	t.Run("a new path no one selected yet is selected", func(t *testing.T) {
		assertStagedRenameSelection(t, map[string]bool{}, []gitrepo.SelectedFile{{Path: "b.go", Language: "go"}}, 1)
	})
	t.Run("a new path the committed diff already selected is not selected twice", func(t *testing.T) {
		assertStagedRenameSelection(t, map[string]bool{"b.go": true}, nil, 0)
	})
}

func assertStagedRenameSelection(t *testing.T, seen map[string]bool, wantFiles []gitrepo.SelectedFile, wantDiags int) {
	t.Helper()
	renames := []gitrepo.WorktreeEntry{{Code: "R ", Path: "b.go", OldPath: "a.go"}}
	wantPair := []codesignal.PathContinuity{{Path: "b.go", PreviousPath: "a.go"}}

	files, diagnostics, continuity := selectStagedRenames(renames, seen, "abc123")

	if !reflect.DeepEqual(files, wantFiles) {
		t.Errorf("files = %+v, want %+v", files, wantFiles)
	}
	if len(diagnostics) != wantDiags {
		t.Errorf("diagnostics = %+v, want %d", diagnostics, wantDiags)
	}
	if !reflect.DeepEqual(continuity, wantPair) {
		t.Errorf("continuity = %+v, want %+v: the pair is recorded whether or not the path is selected again", continuity, wantPair)
	}
}
