package gitrepo

import (
	"strings"
)

type WorktreeEntry struct {
	Code string
	Path string
}

// Dirty-worktree status boundary budgets, mirroring projectconfig's
// read budgets and revisionfs's snapshot budgets: `git status` on
// a large or pathological worktree must fail closed instead of hanging the
// CLI or exhausting memory, the same as every other git read this package
// performs.
const (
	maxWorktreeStatusBytes  = WorktreeStatusBytes
	maxWorktreeStatusStderr = GitStderrBytes
)

// runWorktreeStatusGit is the git seam used by WorktreeStatus. Tests may
// replace it to exercise timeout and bound failures without hanging.
var runWorktreeStatusGit = BoundedRunner(maxWorktreeStatusBytes, maxWorktreeStatusStderr, WorktreeStatusTimeout)

// WorktreeStatus lists every changed or untracked worktree path via
// `git status --porcelain=v1 --untracked-files=all -z`.
func WorktreeStatus(dir string) ([]WorktreeEntry, error) {
	supported, _ := noLazyFetchSupported(runWorktreeStatusGit, dir)
	output, err := runWorktreeStatusGit(dir, worktreeStatusArguments(supported)...)
	if err != nil {
		return nil, err
	}
	return ParseWorktreeStatus(output), nil
}

// worktreeStatusArguments are the arguments of the status. A git that can be told not to fetch is, and
// `git status` then starts a nested status in every populated submodule with that instruction in its environment.
// An older git ignores the instruction, and a submodule that is a partial clone lacking objects is fetched into
// by that nested status: the remote's transport runs, though the guard only looked at the superproject's
// revisions. So that git does not go into submodules, and a change inside one does not show in the status.
func worktreeStatusArguments(canBeToldNotToFetch bool) []string {
	arguments := []string{"status", "--porcelain=v1", "--untracked-files=all", "-z"}
	if !canBeToldNotToFetch {
		arguments = append(arguments, "--ignore-submodules=all")
	}
	return arguments
}

// ParseWorktreeStatus parses `git status --porcelain=v1 -z` output.
// Rename/copy records emit two NUL-delimited fields (new path, then old
// path); the old path is consumed and discarded since only path identity,
// never diff content, is used by any caller.
func ParseWorktreeStatus(output []byte) []WorktreeEntry {
	fields := SplitNULPaths(output)
	entries := make([]WorktreeEntry, 0, len(fields))
	for i := 0; i < len(fields); {
		raw := fields[i]
		i++
		if len(raw) < 3 {
			continue
		}
		code := raw[:2]
		entries = append(entries, WorktreeEntry{Code: code, Path: raw[3:]})
		if strings.ContainsAny(code, "RC") && i < len(fields) {
			i++
		}
	}
	return entries
}
