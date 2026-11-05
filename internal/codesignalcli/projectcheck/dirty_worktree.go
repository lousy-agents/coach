package projectcheck

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// detectRelevantDirtyWorktree lists uncommitted/untracked paths -- via `git
// status`, path names only, never their content -- relevant to the
// readiness result: see IsRelevantDirtyPath for what counts as relevant.
// detectRelevantDirtyWorktree lists the uncommitted paths the report says it did
// not read. readFromDisk is a path the checks did read from the working tree,
// so a change to it is never one of them.
func detectRelevantDirtyWorktree(dir string, roots []string, policyPath, readFromDisk string) (projectreadiness.DirtyWorktree, error) {
	entries, err := gitrepo.WorktreeStatus(dir)
	if err != nil {
		return projectreadiness.DirtyWorktree{}, &gitrepo.OperationalError{Message: fmt.Sprintf("coach codesignal --check-project: git status failed: %s", err)}
	}

	relevant := make([]string, 0, len(entries))
	for _, entry := range entries {
		if readFromDisk != "" && entry.Path == readFromDisk {
			continue
		}
		if IsRelevantDirtyPath(entry.Path, roots, policyPath) {
			relevant = append(relevant, entry.Path)
		}
	}
	sort.Strings(relevant)

	return projectreadiness.DirtyWorktree{RelevantChanges: len(relevant) > 0, Paths: relevant}, nil
}

var alwaysRelevantMetadataBasenames = map[string]bool{
	"package.json":        true,
	"package-lock.json":   true,
	"yarn.lock":           true,
	"pnpm-lock.yaml":      true,
	"npm-shrinkwrap.json": true,
	"bun.lock":            true,
	"bun.lockb":           true,
	".npmrc":              true,
	"bunfig.toml":         true,
}

func IsRelevantDirtyPath(candidate string, roots []string, policyPath string) bool {
	if candidate == policyPath {
		return true
	}
	base := path.Base(candidate)
	if strings.HasPrefix(base, "tsconfig") {
		return true
	}
	if alwaysRelevantMetadataBasenames[base] {
		return true
	}
	for _, root := range roots {
		if pathUnderRoot(candidate, root) {
			return true
		}
	}
	return false
}

func pathUnderRoot(candidate, root string) bool {
	if root == "." || root == "" {
		return true
	}
	return candidate == root || strings.HasPrefix(candidate, root+"/")
}
