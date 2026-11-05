package codesignalcli

import (
	"fmt"
	"slices"
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
	if cause := skippedSubmodulesCause(v.status.SubmodulesSkippedBy, gitlinks); cause != "" {
		causes = append(causes, cause)
	}
	return causes, nil
}

// skippedSubmodulesCause names the submodules a status that did not look into them could not see changes in. A git
// that cannot be told not to fetch is given that status, because a nested status in a submodule that is a partial
// clone would run its remote's transport, and a clean verdict over a submodule with edits would be a false all-clear.
func skippedSubmodulesCause(skippedBy string, gitlinks []string) string {
	if skippedBy == "" || len(gitlinks) == 0 {
		return ""
	}
	sample, truncated := worktreePathSample(slices.Sorted(slices.Values(gitlinks)))
	if truncated {
		sample += ", …"
	}
	return fmt.Sprintf("git %s cannot be told not to fetch, so its status did not look into the submodules (%s), and the worktree scan does not see changes inside them; use git 2.44 or later", skippedBy, sample)
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
