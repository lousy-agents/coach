package gitrepo

import (
	"regexp"
	"strconv"
	"strings"
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
	maxWorktreeStatusBytes  = WorktreeStatusBytes
	maxWorktreeStatusStderr = GitStderrBytes
)

// worktreeStatusGit is the git every status read is made with, held to the budget of the status listing.
func worktreeStatusGit() Runner {
	return BoundedRunner(maxWorktreeStatusBytes, maxWorktreeStatusStderr, WorktreeStatusTimeout)
}

// CommittedWorktreeStatus lists every changed or untracked worktree path via
// `git status --porcelain=v1 --untracked-files=all -z`, for a committed scan. ReadWorktreeStatus is the read that a
// --worktree scan makes, which also keeps the hooks directory and the system-wide configuration out of it and looks
// into every submodule.
func CommittedWorktreeStatus(dir string) ([]WorktreeEntry, error) {
	return worktreeStatusBy(worktreeStatusGit(), dir)
}

// worktreeStatusBy is CommittedWorktreeStatus with the git that answers given by the caller, so that a test can record
// the questions it is asked, or hand in one that times out, without replacing anything.
func worktreeStatusBy(run Runner, dir string) ([]WorktreeEntry, error) {
	supported, version := noLazyFetchSupported(run, dir)
	return worktreeStatusOf(run, dir, supported, version)
}

// worktreeStatusOf is the status for a git of the given version, which can, or cannot, be told not to fetch.
func worktreeStatusOf(run Runner, dir string, canBeToldNotToFetch bool, version string) ([]WorktreeEntry, error) {
	output, err := run(dir, worktreeStatusArguments(canBeToldNotToFetch, version)...)
	if err != nil {
		return nil, err
	}
	return ParseWorktreeStatus(output), nil
}

// runWorktreeStatusGit runs a read of a --worktree scan and returns its stdout, held to the budget of the status
// listing.
func runWorktreeStatusGit(dir string, args ...string) ([]byte, error) {
	return RunBytesBoundedWith(worktreeGitCommand, dir, maxWorktreeStatusBytes, maxWorktreeStatusStderr, WorktreeStatusTimeout, args...)
}

// WorktreeStatusRead is one read of `git status`: its entries, and what git
// warned about on stderr while it succeeded. A directory git could not open is
// left out of the entries without failing the command, so without the warning a
// tree with part unread looks the same as a tree with nothing to report.
type WorktreeStatusRead struct {
	Entries  []WorktreeEntry
	Warnings StatusWarnings
	// SubmodulesSkippedBy is the version of a git that cannot be told not to fetch, which is given the status that
	// does not look into submodules (see worktreeScanStatusArguments), so a change inside one is not in Entries. It is
	// empty when the status looked into them.
	SubmodulesSkippedBy string
}

// ReadWorktreeStatus lists every changed or untracked worktree path via
// `git status --porcelain=v1 --untracked-files=all -z`, with git's warnings.
func ReadWorktreeStatus(dir string) (WorktreeStatusRead, error) {
	supported, version := noLazyFetchSupported(worktreeStatusGit(), dir)
	return readWorktreeStatusBy(worktreeGitCommand, dir, supported, version)
}

// readWorktreeStatusBy is ReadWorktreeStatus with the git that answers, and the version it is, given by the caller,
// so that a test can run the status as a git that cannot be told not to fetch runs it.
func readWorktreeStatusBy(build childBuilder, dir string, canBeToldNotToFetch bool, version string) (WorktreeStatusRead, error) {
	stdout, stderr, err := RunBytesBoundedWithStderr(build, dir, maxWorktreeStatusBytes, maxWorktreeStatusStderr, WorktreeStatusTimeout,
		worktreeScanStatusArguments(canBeToldNotToFetch, version)...)
	if err != nil {
		return WorktreeStatusRead{}, err
	}
	read := WorktreeStatusRead{Entries: ParseWorktreeStatus(stdout), Warnings: ParseStatusWarnings(stderr)}
	if !canBeToldNotToFetch {
		read.SubmodulesSkippedBy = version
	}
	return read, nil
}

// worktreeScanStatusArguments are the arguments of the status a --worktree scan reads. A git that can be told not to
// fetch inspects every submodule, whatever the repository's .gitmodules says: its ignore setting would otherwise let
// a hostile repository hide a dirty submodule from the scan, and renames are detected, because the scan follows a
// renamed file to its source. A git that cannot be told not to fetch is given the arguments every other status gets
// (see worktreeStatusArguments), because a nested status in a submodule that is a partial clone, and a comparison
// of a staged deletion with a staged addition, run the remote's transport there.
func worktreeScanStatusArguments(canBeToldNotToFetch bool, version string) []string {
	if !canBeToldNotToFetch {
		return worktreeStatusArguments(false, version)
	}
	return []string{"status", "--porcelain=v1", "--untracked-files=all", "-z", "--ignore-submodules=none"}
}

// worktreeStatusArguments are the arguments of the status.
//
// A git that can be told not to fetch is, and `git status` then starts a nested status in every populated submodule
// with that instruction in its environment. An older git ignores the instruction, and a submodule that is a partial
// clone lacking objects is fetched into by that nested status: the remote's transport runs, though the guard only
// looked at the superproject's revisions. So that git does not go into submodules, and a change inside one does not
// show in the status.
//
// `--no-renames` keeps the status from comparing a staged deletion with a staged addition, which reads the blob of
// each. The guard lists the objects the revisions reach, and an index can name a blob that none of them does and that
// the repository lacks (a path reset from a fetched branch); an older git fetches it, and a current one fails the
// whole status for it. Every path the status lists is still listed, and a staged rename is listed as its deletion and
// its addition. The flag is git 2.18's, so a git that is older, or whose version is not known, is not given it.
func worktreeStatusArguments(canBeToldNotToFetch bool, version string) []string {
	arguments := []string{"status", "--porcelain=v1", "--untracked-files=all", "-z"}
	if statusKnowsNoRenames(version) {
		arguments = append(arguments, "--no-renames")
	}
	if !canBeToldNotToFetch {
		arguments = append(arguments, "--ignore-submodules=all")
	}
	return arguments
}

var statusVersionPattern = regexp.MustCompile(`^(\d+)\.(\d+)`)

// statusKnowsNoRenames is whether the version of git has `git status --no-renames`, which came in 2.18.
func statusKnowsNoRenames(version string) bool {
	match := statusVersionPattern.FindStringSubmatch(version)
	if match == nil {
		return false
	}
	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	return major > 2 || (major == 2 && minor >= 18)
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
