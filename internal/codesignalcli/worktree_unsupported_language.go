package codesignalcli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// unsupportedLanguageDiagnostics names each file the working tree changed that
// the scan cannot analyze, the way a committed diff names a committed change to
// one. A project configuration file is left out: the analysis reads it, and
// scope.worktree.config_files says so. A project policy file is left out too:
// it is not source in an unsupported language, and the dirty-tree disclosure
// lists it because the scan did not read its working-tree bytes. A directory
// entry, a path under node_modules, a path the committed diff already selected,
// and a new path the working tree deleted are left out as well.
func unsupportedLanguageDiagnostics(entries []gitrepo.WorktreeEntry, seen map[string]bool) []codesignal.Diagnostic {
	var diagnostics []codesignal.Diagnostic
	reported := map[string]bool{}
	for _, entry := range entries {
		if reported[entry.Path] || notNamedForLanguage(entry, seen) {
			continue
		}
		if _, supported := semantics.LanguageForExtension(filepath.Ext(entry.Path)); supported {
			continue
		}
		reported[entry.Path] = true
		diagnostics = append(diagnostics, codesignal.Diagnostic{
			Kind:    "unsupported_language",
			Path:    entry.Path,
			Message: fmt.Sprintf("file extension %q is not a supported language", filepath.Ext(entry.Path)),
		})
	}
	return diagnostics
}

func notNamedForLanguage(entry gitrepo.WorktreeEntry, seen map[string]bool) bool {
	return entry.Path == "" || strings.HasSuffix(entry.Path, "/") || seen[entry.Path] || withinNodeModules(entry.Path) ||
		projectmodel.IsConfigInput(entry.Path) || isProjectPolicyFile(entry.Path) || entry.Code == "!!" || gitrepo.NewPathDeletedFromDisk(entry.Code)
}

func isProjectPolicyFile(name string) bool {
	return name == projectconfig.CanonicalPath || name == projectconfig.DefaultPath
}

// newUnsupportedPaths lists, once each, the new files of the working tree (those
// untracked, added, or the destination of a rename) whose language the scan
// cannot analyze. A file removed from the index and kept on disk is not new:
// HEAD holds it.
func newUnsupportedPaths(entries []gitrepo.WorktreeEntry) []string {
	removedFromIndex := removedFromIndexPaths(entries)
	listed := map[string]bool{}
	var names []string
	for _, entry := range entries {
		if !entry.IsNew() || listed[entry.Path] || removedFromIndex[entry.Path] || strings.HasSuffix(entry.Path, "/") || withinNodeModules(entry.Path) {
			continue
		}
		if _, supported := semantics.LanguageForExtension(filepath.Ext(entry.Path)); !supported {
			listed[entry.Path] = true
			names = append(names, entry.Path)
		}
	}
	return names
}
