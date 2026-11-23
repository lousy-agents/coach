package codesignalcli

import (
	"path/filepath"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// ownPathVerdicts says, for each root that is another repository, whether the
// scan would have analyzed the root's own path as source. What is inside the
// repository is never analyzed, so a root hides in-scope source only when its
// name is a supported source path the scope filter keeps. The filter runs once
// for all of them, and a filter that cannot answer leaves the root unknown.
func (v *WorktreeView) ownPathVerdicts(roots []string) map[string]string {
	verdicts := make(map[string]string, len(roots))
	var sources []gitrepo.SelectedFile
	for _, root := range roots {
		verdicts[root] = codesignal.HidesInScopeNo
		if language, ok := semantics.LanguageForExtension(filepath.Ext(root)); ok {
			sources = append(sources, gitrepo.SelectedFile{Path: root, Language: language})
		}
	}
	if len(sources) == 0 {
		return verdicts
	}
	kept, answered := v.keptSources(sources)
	for _, source := range sources {
		verdicts[source.Path] = sourceVerdict([]gitrepo.SelectedFile{source}, kept, answered)
	}
	return verdicts
}
