package projectcheck

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// detectRelevantDirtyWorktree lists the uncommitted paths the report says it did
// not read, via `git status`: path names only, never their content. See
// IsRelevantDirtyPath for what counts as relevant. The policy's own path is never
// one of them: the check read it from the working tree.
func detectRelevantDirtyWorktree(dir string, roots []string, policyPath string) (projectreadiness.DirtyWorktree, error) {
	entries, err := gitrepo.CommittedWorktreeStatus(dir)
	if err != nil {
		return projectreadiness.DirtyWorktree{}, &gitrepo.OperationalError{Message: fmt.Sprintf("coach codesignal --check-project: git status failed: %s", err)}
	}

	relevant := relevantDirtyPaths(entries, roots, policyPath)

	return projectreadiness.DirtyWorktree{RelevantChanges: len(relevant) > 0, Paths: relevant}, nil
}

// relevantDirtyPaths are the paths of entries the report did not read and that count as relevant, sorted.
// A path is the policy's own when it names the same path as policyPath, whichever Unicode form git lists it in and
// the user typed it in.
func relevantDirtyPaths(entries []gitrepo.WorktreeEntry, roots []string, policyPath string) []string {
	relevant := make([]string, 0, len(entries))
	for _, entry := range entries {
		if gitrepo.SamePath(entry.Path, policyPath) {
			continue
		}
		if IsRelevantDirtyPath(entry.Path, roots, policyPath) {
			relevant = append(relevant, entry.Path)
		}
	}
	sort.Strings(relevant)
	return relevant
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
	if gitrepo.SamePath(candidate, policyPath) {
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
