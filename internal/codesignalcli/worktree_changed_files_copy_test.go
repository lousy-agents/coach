package codesignalcli

import (
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// git status lists a copy only under status.renames=copies, and an end-to-end
// scan reaches the copy's path without this branch, so the branch is pinned at
// the function.
func TestChangedFilesFromStatusSelectsTheNewPathOfACopyAsAdded(t *testing.T) {
	for _, code := range []string{"C ", "CM"} {
		t.Run(code, func(t *testing.T) {
			assertCopySelectedAsAdded(t, code)
		})
	}
}

func assertCopySelectedAsAdded(t *testing.T, code string) {
	t.Helper()
	entries := []gitrepo.WorktreeEntry{{Code: code, Path: "pkg/copy.go", OldPath: "pkg/original.go"}}

	got := changedFilesFromStatus(entries, nil)

	want := []gitrepo.SelectedFile{{Path: "pkg/copy.go", Status: codesignal.ChangeStatus("added"), Language: semantics.LanguageGo}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("changedFilesFromStatus(%q) = %+v, want %+v", code, got, want)
	}
}
