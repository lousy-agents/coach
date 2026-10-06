package gitrepo

import (
	"strings"
	"time"
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
	maxWorktreeStatusBytes  = 16 << 20
	maxWorktreeStatusStderr = 64 << 10
	worktreeStatusTimeout   = 30 * time.Second
)

// runWorktreeStatusGit is the git seam used by WorktreeStatus. Tests may
// replace it to exercise timeout and bound failures without hanging.
var runWorktreeStatusGit = func(dir string, args ...string) ([]byte, error) {
	return RunBytesBounded(dir, maxWorktreeStatusBytes, maxWorktreeStatusStderr, worktreeStatusTimeout, args...)
}

// WorktreeStatus lists every changed or untracked worktree path via
// `git status --porcelain=v1 --untracked-files=all -z`.
func WorktreeStatus(dir string) ([]WorktreeEntry, error) {
	output, err := runWorktreeStatusGit(dir, "status", "--porcelain=v1", "--untracked-files=all", "-z")
	if err != nil {
		return nil, err
	}
	return ParseWorktreeStatus(output), nil
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
