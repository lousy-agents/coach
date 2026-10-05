package codesignalcli

import (
	"fmt"
	"sort"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// detectRelevantDirtyWorktree lists uncommitted/untracked paths -- via `git
// status`, path names only, never their content -- relevant to the
// readiness result: see isRelevantDirtyPath for what counts as relevant.
func detectRelevantDirtyWorktree(dir string, roots []string, policyPath string) (projectreadiness.DirtyWorktree, error) {
	entries, err := gitrepo.WorktreeStatus(dir)
	if err != nil {
		return projectreadiness.DirtyWorktree{}, &gitrepo.OperationalError{Message: fmt.Sprintf("coach codesignal --check-project: git status failed: %s", err)}
	}

	relevant := make([]string, 0, len(entries))
	for _, entry := range entries {
		if isRelevantDirtyPath(entry.Path, roots, policyPath) {
			relevant = append(relevant, entry.Path)
		}
	}
	sort.Strings(relevant)

	return projectreadiness.DirtyWorktree{RelevantChanges: len(relevant) > 0, Paths: relevant}, nil
}
