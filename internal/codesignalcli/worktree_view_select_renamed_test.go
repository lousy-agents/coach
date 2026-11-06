package codesignalcli

import (
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func committedRename(dest, source string) (gitrepo.SelectedFile, []codesignal.Diagnostic, []codesignal.PathContinuity) {
	return gitrepo.SelectedFile{Path: dest, Language: semantics.LanguageGo, Source: source},
		[]codesignal.Diagnostic{{Kind: "continuity_not_determined", Path: dest}},
		[]codesignal.PathContinuity{{Path: dest, PreviousPath: source}}
}

func TestSelectDiffRemovesTheSourceOfACommittedRenameWhoseNewPathWasDeleted(t *testing.T) {
	file, diagnostics, continuity := committedRename("pkg/r.go", "pkg/m.go")
	view := selectionViewOverFiles(t, " D pkg/r.go", fstest.MapFS{})

	selection, err := view.SelectDiff("HEAD", []gitrepo.SelectedFile{file}, diagnostics, continuity, dropTestFiles)

	if err != nil {
		t.Fatal(err)
	}
	wantFiles := []gitrepo.SelectedFile{{Path: "pkg/m.go", Status: "removed", Language: semantics.LanguageGo}}
	if !reflect.DeepEqual(selection.Files, wantFiles) {
		t.Errorf("Files = %+v, want only the old name, removed: %+v", selection.Files, wantFiles)
	}
	if len(selection.Continuity) != 0 || countKind(selection.Diagnostics, "continuity_not_determined") != 0 {
		t.Errorf("continuity %+v and diagnostics %+v must not describe a rename with nothing at the new path", selection.Continuity, selection.Diagnostics)
	}
}

func TestSelectDiffKeepsACommittedRenameWhoseNewPathIsOnDisk(t *testing.T) {
	file, diagnostics, continuity := committedRename("pkg/r.go", "pkg/m.go")
	view := selectionViewOverFiles(t, " M pkg/r.go", fstest.MapFS{"pkg/r.go": {Data: []byte("package pkg\n")}})

	selection, err := view.SelectDiff("HEAD", []gitrepo.SelectedFile{file}, diagnostics, continuity, dropTestFiles)

	if err != nil {
		t.Fatal(err)
	}
	if len(selection.Files) != 1 || selection.Files[0].Path != "pkg/r.go" {
		t.Errorf("Files = %+v, want the renamed file", selection.Files)
	}
	if !reflect.DeepEqual(selection.Continuity, continuity) || countKind(selection.Diagnostics, "continuity_not_determined") != 1 {
		t.Errorf("continuity %+v and diagnostics %+v must keep the rename", selection.Continuity, selection.Diagnostics)
	}
}

// A copy leaves its source in place, so deleting the copy removes nothing the
// merge base holds.
func TestSelectDiffDoesNotRemoveTheSourceOfADeletedCopy(t *testing.T) {
	file, diagnostics, continuity := committedRename("pkg/copy.go", "pkg/src.go")
	file.Source = ""
	view := selectionViewOverFiles(t, " D pkg/copy.go", fstest.MapFS{})

	selection, err := view.SelectDiff("HEAD", []gitrepo.SelectedFile{file}, diagnostics, continuity, dropTestFiles)

	if err != nil {
		t.Fatal(err)
	}
	if len(selection.Files) != 1 || selection.Files[0].Path != "pkg/copy.go" {
		t.Errorf("Files = %+v, want only the copy, as before: the source of a copy is not removed", selection.Files)
	}
}

func TestSelectDiffSelectsNothingForARenamedFromALanguageItDoesNotAnalyze(t *testing.T) {
	file, diagnostics, continuity := committedRename("pkg/r.go", "pkg/m.txt")
	view := selectionViewOverFiles(t, " D pkg/r.go", fstest.MapFS{})

	selection, err := view.SelectDiff("HEAD", []gitrepo.SelectedFile{file}, diagnostics, continuity, dropTestFiles)

	if err != nil {
		t.Fatal(err)
	}
	if len(selection.Files) != 0 {
		t.Errorf("Files = %+v, want none", selection.Files)
	}
}
