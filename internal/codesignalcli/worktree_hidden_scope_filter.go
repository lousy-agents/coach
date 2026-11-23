package codesignalcli

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// keptSources reports false when the scope filter is unavailable or fails, so a
// scan that cannot tell does not read as complete.
func (v *WorktreeView) keptSources(sources []gitrepo.SelectedFile) (map[string]bool, bool) {
	if v.scope == nil {
		return nil, false
	}
	kept, err := v.scope(sources)
	if err != nil {
		return nil, false
	}
	paths := make(map[string]bool, len(kept))
	for _, file := range kept {
		paths[file.Path] = true
	}
	return paths, true
}

func sourceVerdict(sources []gitrepo.SelectedFile, kept map[string]bool, filterAnswered bool) string {
	if !filterAnswered {
		return codesignal.HidesInScopeUnknown
	}
	for _, source := range sources {
		if kept[source.Path] {
			return codesignal.HidesInScopeYes
		}
	}
	return codesignal.HidesInScopeNo
}
