package gitrepo_test

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

func TestWorktreeEntryReadsAStatusCodeAsAMeaningAboutItsPath(t *testing.T) {
	tests := []struct {
		name                string
		entry               gitrepo.WorktreeEntry
		untracked, isNew    bool
		edited, deletedDisk bool
		renameSourceGone    bool
		unmerged            bool
	}{
		{name: "an untracked file", entry: gitrepo.WorktreeEntry{Code: "??", Path: "a.go"}, untracked: true, isNew: true},
		{name: "a staged addition", entry: gitrepo.WorktreeEntry{Code: "A ", Path: "a.go"}, isNew: true},
		{name: "an intent-to-add file", entry: gitrepo.WorktreeEntry{Code: " A", Path: "a.go"}, isNew: true},
		{name: "the new path of a staged rename", entry: gitrepo.WorktreeEntry{Code: "R ", Path: "new.go", OldPath: "old.go"}, isNew: true},
		{name: "the new path of a staged copy", entry: gitrepo.WorktreeEntry{Code: "C ", Path: "copy.go", OldPath: "src.go"}, isNew: true},
		{name: "an unstaged edit", entry: gitrepo.WorktreeEntry{Code: " M", Path: "a.go"}, edited: true},
		{name: "a staged edit", entry: gitrepo.WorktreeEntry{Code: "M ", Path: "a.go"}, edited: true},
		{name: "a staged and unstaged edit", entry: gitrepo.WorktreeEntry{Code: "MM", Path: "a.go"}, edited: true},
		{name: "an addition edited afterwards", entry: gitrepo.WorktreeEntry{Code: "AM", Path: "a.go"}, isNew: true},
		{name: "a rename edited afterwards", entry: gitrepo.WorktreeEntry{Code: "RM", Path: "new.go", OldPath: "old.go"}, isNew: true},
		{name: "a copy edited afterwards", entry: gitrepo.WorktreeEntry{Code: "CM", Path: "copy.go", OldPath: "src.go"}, isNew: true},
		{name: "a staged edit that is then a type change", entry: gitrepo.WorktreeEntry{Code: "MT", Path: "a.go"}, edited: true},
		{name: "a type change", entry: gitrepo.WorktreeEntry{Code: " T", Path: "a.go"}, edited: true},
		{name: "an unstaged deletion", entry: gitrepo.WorktreeEntry{Code: " D", Path: "a.go"}, deletedDisk: true},
		{name: "a staged deletion", entry: gitrepo.WorktreeEntry{Code: "D ", Path: "a.go"}, deletedDisk: true},
		{name: "a staged edit then deleted", entry: gitrepo.WorktreeEntry{Code: "MD", Path: "a.go"}, deletedDisk: true},
		{name: "an addition then deleted was never in HEAD", entry: gitrepo.WorktreeEntry{Code: "AD", Path: "a.go"}},
		{name: "a rename whose new path was then deleted", entry: gitrepo.WorktreeEntry{Code: "RD", Path: "new.go", OldPath: "old.go"}, renameSourceGone: true},
		{name: "a copy whose new path was then deleted leaves its source", entry: gitrepo.WorktreeEntry{Code: "CD", Path: "copy.go", OldPath: "src.go"}},
		{name: "an unmerged file is not an edit", entry: gitrepo.WorktreeEntry{Code: "UU", Path: "a.go"}, unmerged: true},
		{name: "a file both sides added is unmerged, and HEAD holds it, so it is not new", entry: gitrepo.WorktreeEntry{Code: "AA", Path: "a.go"}, unmerged: true},
		{name: "a file added by us and conflicted is unmerged, and HEAD holds it, so it is not new", entry: gitrepo.WorktreeEntry{Code: "AU", Path: "a.go"}, unmerged: true},
		{name: "a file added by them and conflicted is unmerged", entry: gitrepo.WorktreeEntry{Code: "UA", Path: "a.go"}, unmerged: true},
		{name: "a file deleted by us and conflicted is unmerged", entry: gitrepo.WorktreeEntry{Code: "DU", Path: "a.go"}, unmerged: true},
		{name: "a file deleted by them and conflicted is unmerged", entry: gitrepo.WorktreeEntry{Code: "UD", Path: "a.go"}, unmerged: true},
		{name: "a file both sides deleted is not an edit and has no single answer", entry: gitrepo.WorktreeEntry{Code: "DD", Path: "a.go"}, unmerged: true},
		{name: "an ignored path is nothing", entry: gitrepo.WorktreeEntry{Code: "!!", Path: "a.log"}},
		{name: "a record too short to carry a code", entry: gitrepo.WorktreeEntry{Code: "M", Path: "a.go"}},
		{name: "a blank code changes nothing", entry: gitrepo.WorktreeEntry{Code: "  ", Path: "a.go"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertEntryMeaning(t, tt.entry, map[string]bool{
				"IsUntracked":       tt.untracked,
				"IsNew":             tt.isNew,
				"IsEditedTracked":   tt.edited,
				"IsDeletedFromDisk": tt.deletedDisk,
				"RenameSourceGone":  tt.renameSourceGone,
				"IsUnmerged":        tt.unmerged,
			})
		})
	}
}

func assertEntryMeaning(t *testing.T, entry gitrepo.WorktreeEntry, want map[string]bool) {
	t.Helper()
	got := map[string]bool{
		"IsUntracked":       entry.IsUntracked(),
		"IsNew":             entry.IsNew(),
		"IsEditedTracked":   entry.IsEditedTracked(),
		"IsDeletedFromDisk": entry.IsDeletedFromDisk(),
		"RenameSourceGone":  entry.RenameSourceGone(),
		"IsUnmerged":        entry.IsUnmerged(),
	}
	for method, expected := range want {
		if got[method] != expected {
			t.Errorf("%+v.%s() = %v, want %v", entry, method, got[method], expected)
		}
	}
}

func TestWorktreeEntryNamesTheSourceOfARenameAndNoOtherEntry(t *testing.T) {
	tests := []struct {
		name       string
		entry      gitrepo.WorktreeEntry
		wantSource string
		wantMoved  bool
	}{
		{"a staged rename", gitrepo.WorktreeEntry{Code: "R ", Path: "new.go", OldPath: "old.go"}, "old.go", true},
		{"a rename edited afterwards", gitrepo.WorktreeEntry{Code: "RM", Path: "new.go", OldPath: "old.go"}, "old.go", true},
		{"an intent-to-add rename", gitrepo.WorktreeEntry{Code: " R", Path: "new.go", OldPath: "old.go"}, "old.go", true},
		{"a rename whose new path was deleted", gitrepo.WorktreeEntry{Code: "RD", Path: "new.go", OldPath: "old.go"}, "old.go", true},
		{"a copy keeps its source", gitrepo.WorktreeEntry{Code: "C ", Path: "copy.go", OldPath: "src.go"}, "src.go", false},
		{"an edit", gitrepo.WorktreeEntry{Code: " M", Path: "a.go"}, "", false},
		{"a rename record without a source", gitrepo.WorktreeEntry{Code: "R ", Path: "new.go"}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertRenameSource(t, tt.entry, tt.wantSource, tt.wantMoved)
		})
	}
}

func assertRenameSource(t *testing.T, entry gitrepo.WorktreeEntry, wantSource string, wantMoved bool) {
	t.Helper()
	if source, moved := entry.RenameSource(); moved != wantMoved || (moved && source != wantSource) {
		t.Errorf("RenameSource() = %q, %v; want %q, %v", source, moved, wantSource, wantMoved)
	}
}
