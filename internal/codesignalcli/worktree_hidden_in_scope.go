package codesignalcli

import (
	"path/filepath"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// hidesInScope says whether any hidden path is one the scan would have analyzed:
// a project configuration file the overlay reads, the policy file the user named, or
// source the scan's own scope filter keeps.
func (v *WorktreeView) hidesInScope(hidden []string) string {
	var sources []gitrepo.SelectedFile
	for _, name := range hidden {
		if name == v.namedPolicy || (!withinNodeModules(name) && projectmodel.IsConfigInput(name)) {
			return codesignal.HidesInScopeYes
		}
		if language, ok := semantics.LanguageForExtension(filepath.Ext(name)); ok && !withinNodeModules(name) {
			sources = append(sources, gitrepo.SelectedFile{Path: name, Language: language})
		}
	}
	if len(sources) == 0 {
		return codesignal.HidesInScopeNo
	}
	return v.sourceInScope(sources)
}

// sourceInScope is "unknown" when the scope filter is unavailable or fails, so a
// scan that cannot tell does not read as complete.
func (v *WorktreeView) sourceInScope(sources []gitrepo.SelectedFile) string {
	if v.scope == nil {
		return codesignal.HidesInScopeUnknown
	}
	kept, err := v.scope(sources)
	switch {
	case err != nil:
		return codesignal.HidesInScopeUnknown
	case len(kept) > 0:
		return codesignal.HidesInScopeYes
	}
	return codesignal.HidesInScopeNo
}
