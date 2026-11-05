package codesignalcli

import (
	"path/filepath"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// changedFilesFromStatus lists the supported source files the working tree
// changes that are not in selected: tracked files edited in the working tree,
// as modified changes, and new files (untracked, staged, or the new path of a
// rename), as added changes. With --worktree "the change" includes both, so a
// diff scan analyzes a file edited only on disk although it is absent from the
// merge-base..HEAD diff, and a baseline scan analyzes files HEAD does not hold.
func changedFilesFromStatus(entries []gitrepo.WorktreeEntry, selected []gitrepo.SelectedFile) []gitrepo.SelectedFile {
	already := make(map[string]bool, len(selected))
	for _, file := range selected {
		already[file.Path] = true
	}
	removedFromIndex := removedFromIndexPaths(entries)
	var changed []gitrepo.SelectedFile
	for _, entry := range entries {
		language, ok := semantics.LanguageForExtension(filepath.Ext(entry.Path))
		if !ok || already[entry.Path] || withinNodeModules(entry.Path) {
			continue
		}
		status, selectable := changeStatusOf(entry)
		if !selectable {
			continue
		}
		if removedFromIndex[entry.Path] {
			status = codesignal.ChangeStatus("modified")
		}
		already[entry.Path] = true
		changed = append(changed, gitrepo.SelectedFile{Path: entry.Path, Status: status, Language: language})
	}
	return changed
}

// changeStatusOf maps a status entry to the change the scan analyzes: modified
// for a tracked edit, added for a new file. Deleted, unmerged and ignored
// entries are not selected.
func changeStatusOf(entry gitrepo.WorktreeEntry) (codesignal.ChangeStatus, bool) {
	switch {
	case entry.IsNew():
		return codesignal.ChangeStatus("added"), true
	case entry.IsEditedTracked():
		return codesignal.ChangeStatus("modified"), true
	}
	return "", false
}

// removedFromIndexPaths is every path git status lists as untracked although HEAD
// holds it: a tracked file taken out of the index with its bytes kept on disk
// (listed as deleted and untracked), and the source of a rename that was written
// again as a new file. HEAD holds the path, so reading it from disk is a
// modification, not an addition.
func removedFromIndexPaths(entries []gitrepo.WorktreeEntry) map[string]bool {
	untracked := untrackedPaths(entries)
	removed := map[string]bool{}
	for _, entry := range entries {
		if untracked[entry.Path] && entry.IsDeletedFromDisk() {
			removed[entry.Path] = true
		}
		if source, moved := entry.RenameSource(); moved && untracked[source] {
			removed[source] = true
		}
	}
	return removed
}
