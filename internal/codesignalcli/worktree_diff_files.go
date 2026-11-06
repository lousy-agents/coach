package codesignalcli

import (
	"path/filepath"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// diffFiles returns the files a --base --worktree scan analyzes: the
// committed selection, the working tree's new and edited files, the supported
// files the working tree deleted, and staged renames. A file the committed diff
// edited and the working tree then deleted is a deletion, not an edit: nothing
// is left to analyze at head, so its base-side signals are resolved. A tracked
// file the committed diff does not touch but the working tree deleted is a
// deletion too. A staged rename is selected the way a committed one is, with
// its continuity left undetermined, so a moved file's signals are not reported
// as new. A file the committed diff removed and the working tree wrote again is
// a modification against the merge base, not a removal. A file the committed
// diff renamed and the working tree then deleted leaves nothing at either name,
// so the merge base's file at the old name is removed, and the pair is returned
// in vanished so its continuity is not reported.
func (v *WorktreeView) diffFiles(headSHA string, selected []gitrepo.SelectedFile) (files []gitrepo.SelectedFile, diagnostics []codesignal.Diagnostic, continuity []codesignal.PathContinuity, vanished map[string]bool, err error) {
	if v.status.Err != nil {
		return nil, nil, nil, nil, v.status.Err
	}
	entries := v.status.Entries
	renames := stagedRenames(entries)
	changed := withoutRenamedPaths(changedFilesFromStatus(entries, selected), renames)
	deleted := deletedTrackedPaths(entries)
	written := writtenPaths(entries)
	files = make([]gitrepo.SelectedFile, 0, len(selected)+len(changed)+len(deleted))
	seen := make(map[string]bool, len(selected))
	vanished = map[string]bool{}
	for _, file := range selected {
		seen[file.Path] = true
		if _, gone := deleted[file.Path]; gone && file.Source != "" {
			vanished[file.Path] = true
			files = append(files, removedRenameSource(file)...)
			continue
		}
		file.Status = statusInWorkingTree(file, deleted, written)
		files = append(files, file)
	}
	files = append(files, changed...)
	files = append(files, untouchedDeletions(deleted, seen)...)
	renamed, diagnostics, continuity := selectStagedRenames(renames, seen, headSHA)
	diagnostics = append(diagnostics, unsupportedLanguageDiagnostics(entries, seen)...)
	return append(files, renamed...), diagnostics, continuity, vanished, nil
}

// removedRenameSource is the merge base's file at the old name of a committed
// rename whose new path the working tree deleted: removed, so its findings are
// resolved. A source in a language the scan does not analyze selects nothing.
func removedRenameSource(renamed gitrepo.SelectedFile) []gitrepo.SelectedFile {
	language, supported := semantics.LanguageForExtension(filepath.Ext(renamed.Source))
	if !supported {
		return nil
	}
	return []gitrepo.SelectedFile{{Path: renamed.Source, Status: codesignal.ChangeStatus("removed"), Language: language}}
}

// statusInWorkingTree is the change a selected file is, once the working tree is
// read: a committed edit the working tree deleted is a removal, and a committed
// removal the working tree wrote again is an edit against the merge base.
func statusInWorkingTree(file gitrepo.SelectedFile, deleted map[string]semantics.Language, written map[string]bool) codesignal.ChangeStatus {
	_, gone := deleted[file.Path]
	switch {
	case gone && file.Status == "modified":
		return codesignal.ChangeStatus("removed")
	case written[file.Path] && file.Status == "removed":
		return codesignal.ChangeStatus("modified")
	}
	return file.Status
}

// writtenPaths is every path the working tree holds that HEAD does not: untracked
// and added files, and the destination of a rename or copy.
func writtenPaths(entries []gitrepo.WorktreeEntry) map[string]bool {
	written := map[string]bool{}
	for _, entry := range entries {
		if entry.IsNew() {
			written[entry.Path] = true
		}
	}
	return written
}
