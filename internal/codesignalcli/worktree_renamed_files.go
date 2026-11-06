package codesignalcli

import (
	"fmt"
	"path/filepath"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// stagedRenames lists the renames and copies in a status listing, staged or
// intent-to-add, which carry the source path git paired with the new one. A new
// path the working tree then deleted is not listed: there is nothing at it to
// select.
func stagedRenames(entries []gitrepo.WorktreeEntry) []gitrepo.WorktreeEntry {
	var renames []gitrepo.WorktreeEntry
	for _, entry := range entries {
		if entry.OldPath != "" && gitrepo.AddsPath(entry.Code) {
			renames = append(renames, entry)
		}
	}
	return renames
}

// renameSources lists the source path of every rename in a status listing,
// whichever column git reports it in: HEAD holds each, and the move took it off
// disk. A copy leaves its source in place.
func renameSources(entries []gitrepo.WorktreeEntry) []string {
	var sources []string
	for _, entry := range entries {
		if source, moved := entry.RenameSource(); moved {
			sources = append(sources, source)
		}
	}
	return sources
}

func withoutRenamedPaths(files []gitrepo.SelectedFile, renames []gitrepo.WorktreeEntry) []gitrepo.SelectedFile {
	renamed := make(map[string]bool, len(renames))
	for _, rename := range renames {
		renamed[rename.Path] = true
	}
	kept := files[:0:0]
	for _, file := range files {
		if !renamed[file.Path] {
			kept = append(kept, file)
		}
	}
	return kept
}

// selectStagedRenames selects each rename's new path without marking it added,
// which would report a moved file's signals as introduced, and records the
// continuity pair for every rename, like a committed diff does. The pair is
// recorded for unsupported languages too, because project analysis reads the
// whole snapshot.
func selectStagedRenames(renames []gitrepo.WorktreeEntry, seen map[string]bool, headSHA string) ([]gitrepo.SelectedFile, []codesignal.Diagnostic, []codesignal.PathContinuity) {
	var files []gitrepo.SelectedFile
	var diagnostics []codesignal.Diagnostic
	var continuity []codesignal.PathContinuity
	for _, rename := range renames {
		continuity = append(continuity, codesignal.PathContinuity{Path: rename.Path, PreviousPath: rename.OldPath})
		language, supported := semantics.LanguageForExtension(filepath.Ext(rename.Path))
		if !supported || seen[rename.Path] || withinNodeModules(rename.Path) {
			continue
		}
		files = append(files, gitrepo.SelectedFile{Path: rename.Path, Language: language})
		diagnostics = append(diagnostics, codesignal.Diagnostic{
			Kind:     "continuity_not_determined",
			Path:     rename.Path,
			Message:  fmt.Sprintf("rename/copy lifecycle continuity was not determined: the rename is uncommitted, in the working tree over head revision %s", headSHA),
			Side:     "head",
			Revision: headSHA,
		})
	}
	return files, diagnostics, continuity
}
