package codesignalcli

import (
	"path/filepath"
	"slices"
	"sort"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func untouchedDeletions(deleted map[string]semantics.Language, seen map[string]bool) []gitrepo.SelectedFile {
	var files []gitrepo.SelectedFile
	for name, language := range deleted {
		if !seen[name] {
			files = append(files, gitrepo.SelectedFile{Path: name, Status: codesignal.ChangeStatus("removed"), Language: language})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files
}

// deletedTrackedPaths maps each supported file that HEAD holds and the working
// tree no longer has to its language. A file removed from the index but kept on
// disk is still there, listed as untracked, so it is not a deletion.
func deletedTrackedPaths(entries []gitrepo.WorktreeEntry) map[string]semantics.Language {
	deleted := map[string]semantics.Language{}
	for _, name := range deletedPathNames(entries) {
		if language, supported := semantics.LanguageForExtension(filepath.Ext(name)); supported && !withinNodeModules(name) {
			deleted[name] = language
		}
	}
	return deleted
}

// deletedPathNames lists every path of any language that HEAD holds and the
// working tree no longer has: the paths git lists as deleted, and the source of
// a rename whose new path the working tree then deleted.
func deletedPathNames(entries []gitrepo.WorktreeEntry) []string {
	var names []string
	for _, entry := range entries {
		if entry.IsDeletedFromDisk() {
			names = append(names, entry.Path)
		}
		if entry.RenameSourceGone() {
			names = append(names, entry.OldPath)
		}
	}
	return notOnDisk(names, entries)
}

// notOnDisk drops the names git also lists as untracked: they are still on disk.
func notOnDisk(names []string, entries []gitrepo.WorktreeEntry) []string {
	onDisk := untrackedPaths(entries)
	return slices.DeleteFunc(names, func(name string) bool { return onDisk[name] })
}

// untrackedPaths is every path git status lists as untracked. When a path is
// also listed as deleted, the file was removed from the index only.
func untrackedPaths(entries []gitrepo.WorktreeEntry) map[string]bool {
	untracked := map[string]bool{}
	for _, entry := range entries {
		if entry.IsUntracked() {
			untracked[entry.Path] = true
		}
	}
	return untracked
}
