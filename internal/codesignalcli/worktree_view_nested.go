package codesignalcli

import (
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

const (
	submoduleCause        = "a submodule has working-tree changes, so the worktree scan does not analyze them"
	nestedRepositoryCause = "an untracked nested repository is present, so the worktree scan does not analyze it"
)

// nestedStateCauses names the working-tree state the scan cannot read inside
// another repository: a submodule with changes, and an untracked directory that
// holds its own repository, which git status lists with a trailing slash. The
// overlay reads neither, so a clean verdict over them would be a false all-clear.
func (v *WorktreeView) nestedStateCauses(dir string) ([]string, error) {
	entries := v.status.Entries
	gitlinks, err := v.gitlinks(dir)
	if err != nil {
		return nil, err
	}
	submodules := pathSet(gitlinks)
	var causes []string
	if hasDirtySubmodule(entries, submodules) {
		causes = append(causes, submoduleCause)
	}
	if hasUntrackedRepository(entries) {
		causes = append(causes, nestedRepositoryCause)
	}
	return causes, nil
}

func hasDirtySubmodule(entries []gitrepo.WorktreeEntry, submodules map[string]bool) bool {
	for _, entry := range entries {
		if submodules[entry.Path] && !entry.IsDeletedFromDisk() {
			return true
		}
	}
	return false
}

func hasUntrackedRepository(entries []gitrepo.WorktreeEntry) bool {
	for _, entry := range entries {
		if entry.IsUntracked() && strings.HasSuffix(entry.Path, "/") {
			return true
		}
	}
	return false
}
