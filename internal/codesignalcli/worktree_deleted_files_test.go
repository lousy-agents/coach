package codesignalcli

import (
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestUntouchedDeletionsAreRemovedFilesListedByPathAndSkipTheSelectedOnes(t *testing.T) {
	deleted := map[string]semantics.Language{
		"web/old.ts": semantics.LanguageTypeScript,
		"z.go":       semantics.LanguageGo,
		"a.go":       semantics.LanguageGo,
		"kept.go":    semantics.LanguageGo,
	}

	got := untouchedDeletions(deleted, map[string]bool{"kept.go": true})

	want := []gitrepo.SelectedFile{
		{Path: "a.go", Status: codesignal.ChangeStatus("removed"), Language: semantics.LanguageGo},
		{Path: "web/old.ts", Status: codesignal.ChangeStatus("removed"), Language: semantics.LanguageTypeScript},
		{Path: "z.go", Status: codesignal.ChangeStatus("removed"), Language: semantics.LanguageGo},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("untouchedDeletions = %+v, want %+v in path order, without the path the committed diff selected", got, want)
	}
}

func TestDeletedTrackedPathsLeavesOutSourceFilesUnderNodeModules(t *testing.T) {
	entries := []gitrepo.WorktreeEntry{
		{Code: " D", Path: "node_modules/pkg/index.ts"},
		{Code: "D ", Path: "web/node_modules/pkg/lib.go"},
		{Code: " D", Path: "src/old.go"},
		{Code: " D", Path: "docs/notes.txt"},
	}

	got := deletedTrackedPaths(entries)

	want := map[string]semantics.Language{"src/old.go": semantics.LanguageGo}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("deletedTrackedPaths = %v, want %v: a dependency under node_modules and an unsupported file are not the project's deleted source", got, want)
	}
}
