package gitrepo

import (
	"strings"
	"time"
)

type WorktreeEntry struct {
	Code string
	Path string
	// OldPath is the source of a rename or copy record, and empty for any other.
	OldPath string
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

// runWorktreeStatusGit runs a hardened git read and returns its stdout.
func runWorktreeStatusGit(dir string, args ...string) ([]byte, error) {
	stdout, _, err := RunBytesBoundedWithStderr(worktreeGitCommand, dir, maxWorktreeStatusBytes, maxWorktreeStatusStderr, worktreeStatusTimeout, args...)
	return stdout, err
}

// WorktreeStatusRead is one read of `git status`: its entries, and what git
// warned about on stderr while it succeeded. A directory git could not open is
// left out of the entries without failing the command, so without the warning a
// tree with part unread looks the same as a tree with nothing to report.
type WorktreeStatusRead struct {
	Entries  []WorktreeEntry
	Warnings StatusWarnings
}

// ReadWorktreeStatus lists every changed or untracked worktree path via
// `git status --porcelain=v1 --untracked-files=all -z`, with git's warnings.
func ReadWorktreeStatus(dir string) (WorktreeStatusRead, error) {
	stdout, stderr, err := RunBytesBoundedWithStderr(worktreeGitCommand, dir, maxWorktreeStatusBytes, maxWorktreeStatusStderr, worktreeStatusTimeout,
		"status", "--porcelain=v1", "--untracked-files=all", "-z")
	if err != nil {
		return WorktreeStatusRead{}, err
	}
	return WorktreeStatusRead{Entries: ParseWorktreeStatus(stdout), Warnings: ParseStatusWarnings(stderr)}, nil
}

// ParseWorktreeStatus parses `git status --porcelain=v1 -z` output.
// Rename/copy records emit two NUL-delimited fields (new path, then old
// path); the old path is kept in OldPath.
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
		entry := WorktreeEntry{Code: code, Path: raw[3:]}
		if strings.ContainsAny(code, "RC") && i < len(fields) {
			entry.OldPath = fields[i]
			i++
		}
		entries = append(entries, entry)
	}
	return entries
}

// WorktreeStatus lists every changed or untracked worktree path via
// `git status --porcelain=v1 --untracked-files=all -z`.
func WorktreeStatus(dir string) ([]WorktreeEntry, error) {
	read, err := ReadWorktreeStatus(dir)
	return read.Entries, err
}
