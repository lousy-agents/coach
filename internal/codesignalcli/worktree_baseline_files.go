package codesignalcli

import (
	"path/filepath"
	"slices"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// baselineFiles turns the committed revision's discovered files into the
// working tree's: tracked files the working tree deleted are taken out of the
// selection and the coverage, whatever their language or scope, and new
// supported files are added to both. New files of an unsupported language are
// counted under their extension, as the committed result will count them. The
// caller applies the scope filter after this, so a deleted file is never counted
// as excluded.
func (v *WorktreeView) baselineFiles(discovered []gitrepo.SelectedFile, coverage codesignal.Coverage) ([]gitrepo.SelectedFile, codesignal.Coverage, error) {
	if v.status.Err != nil {
		return nil, coverage, v.status.Err
	}
	entries := v.status.Entries
	deleted := deletedPaths(entries)
	files := make([]gitrepo.SelectedFile, 0, len(discovered))
	for _, file := range discovered {
		if deleted[file.Path] {
			coverage.TrackedFilesDiscovered--
			continue
		}
		files = append(files, file)
	}
	for name := range deleted {
		if _, supported := semantics.LanguageForExtension(filepath.Ext(name)); !supported {
			coverage = gitrepo.WithoutUnsupportedFile(coverage, name)
		}
	}
	for _, name := range newUnsupportedPaths(entries) {
		coverage = gitrepo.WithUnsupportedFile(coverage, name)
	}
	added := changedFilesFromStatus(entries, files)
	coverage.TrackedFilesDiscovered += len(added)
	return append(files, added...), coverage, nil
}

// deletedPaths is every path of any language that HEAD holds and the working
// tree no longer has, including the source of a rename: the scan counts the
// rename's destination as a new file, so the source must leave the count.
func deletedPaths(entries []gitrepo.WorktreeEntry) map[string]bool {
	deleted := map[string]bool{}
	for _, name := range slices.Concat(deletedPathNames(entries), notOnDisk(renameSources(entries), entries)) {
		deleted[name] = true
	}
	return deleted
}
