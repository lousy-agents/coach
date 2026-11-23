package codesignalcli

import (
	"path/filepath"
	"slices"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// hidesInScope says, for each root, whether the paths hidden under it include
// one the scan would have analyzed: a project configuration file the overlay
// reads, the policy file the scan selected, or source the scan's own scope
// filter keeps. A root is judged only by its own paths. The scope filter runs
// once for the source of every root that a configuration file or the policy did
// not already decide.
func (v *WorktreeView) hidesInScope(hiddenByRoot map[string][]string, policyPaths ...string) map[string]string {
	verdicts := make(map[string]string, len(hiddenByRoot))
	sourcesByRoot := map[string][]gitrepo.SelectedFile{}
	var allSources []gitrepo.SelectedFile
	for root, hidden := range hiddenByRoot {
		configHidden, sources := classifyHidden(hidden, policyPaths...)
		switch {
		case configHidden:
			verdicts[root] = codesignal.HidesInScopeYes
		case len(sources) == 0:
			verdicts[root] = codesignal.HidesInScopeNo
		default:
			sourcesByRoot[root] = sources
			allSources = append(allSources, sources...)
		}
	}
	if len(sourcesByRoot) == 0 {
		return verdicts
	}
	kept, ok := v.keptSources(allSources)
	for root, sources := range sourcesByRoot {
		verdicts[root] = sourceVerdict(sources, kept, ok)
	}
	return verdicts
}

// classifyHidden separates the hidden paths the overlay or the policy reads
// directly from the source files whose scope is still to be decided.
func classifyHidden(hidden []string, policyPaths ...string) (configHidden bool, sources []gitrepo.SelectedFile) {
	for _, name := range hidden {
		if slices.ContainsFunc(policyPaths, func(policy string) bool { return gitrepo.SamePath(name, policy) }) || (!withinNodeModules(name) && projectmodel.IsConfigInput(name)) {
			return true, nil
		}
		if language, ok := semantics.LanguageForExtension(filepath.Ext(name)); ok && !withinNodeModules(name) {
			sources = append(sources, gitrepo.SelectedFile{Path: name, Language: language})
		}
	}
	return false, sources
}
