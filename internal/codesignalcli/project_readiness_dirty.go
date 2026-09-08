package codesignalcli

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type worktreeStatusEntry struct {
	code string
	path string
}

// Dirty-worktree status boundary budgets, mirroring project.go's
// maxProjectConfig* and project_snapshot.go's maxSnapshot*: `git status` on
// a large or pathological worktree must fail closed instead of hanging the
// CLI or exhausting memory, the same as every other git read this package
// performs.
const (
	maxDirtyWorktreeStatusListingBytes = 16 << 20
	maxDirtyWorktreeGitStderr          = 64 << 10
	dirtyWorktreeGitTimeout            = 30 * time.Second
)

// runDirtyWorktreeGit is the git seam used by gitWorktreeStatus. Tests may
// replace it to exercise timeout and bound failures without hanging.
var runDirtyWorktreeGit = func(dir string, args ...string) ([]byte, error) {
	return runGitBytesBounded(dir, maxDirtyWorktreeStatusListingBytes, maxDirtyWorktreeGitStderr, dirtyWorktreeGitTimeout, args...)
}

// detectRelevantDirtyWorktree lists uncommitted/untracked paths -- via `git
// status`, path names only, never their content -- relevant to the
// readiness result: see isRelevantDirtyPath for what counts as relevant.
func detectRelevantDirtyWorktree(dir string, roots []string, policyPath string) (ReadinessDirtyWorktree, error) {
	entries, err := gitWorktreeStatus(dir)
	if err != nil {
		return ReadinessDirtyWorktree{}, &OperationalError{Message: fmt.Sprintf("coach codesignal --check-project: git status failed: %s", err)}
	}

	relevant := make([]string, 0, len(entries))
	for _, entry := range entries {
		if isRelevantDirtyPath(entry.path, roots, policyPath) {
			relevant = append(relevant, entry.path)
		}
	}
	sort.Strings(relevant)

	return ReadinessDirtyWorktree{RelevantChanges: len(relevant) > 0, Paths: relevant}, nil
}

// gitWorktreeStatus parses `git status --porcelain=v1 --untracked-files=all
// -z`. Rename/copy records emit two NUL-delimited fields (new path, then
// old path); the old path is consumed and discarded since only path
// identity, never diff content, is used by any caller.
func gitWorktreeStatus(dir string) ([]worktreeStatusEntry, error) {
	output, err := runDirtyWorktreeGit(dir, "status", "--porcelain=v1", "--untracked-files=all", "-z")
	if err != nil {
		return nil, err
	}

	fields := splitNULPaths(output)
	entries := make([]worktreeStatusEntry, 0, len(fields))
	for i := 0; i < len(fields); {
		raw := fields[i]
		i++
		if len(raw) < 3 {
			continue
		}
		code := raw[:2]
		entries = append(entries, worktreeStatusEntry{code: code, path: raw[3:]})
		if strings.ContainsAny(code, "RC") && i < len(fields) {
			i++
		}
	}
	return entries, nil
}
