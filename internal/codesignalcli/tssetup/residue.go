package tssetup

import (
	"bytes"
	"path/filepath"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

// repositoryRelativeChangedPaths renders outcome.ChangedPaths for display.
// Ordinarily they are already repository-root-relative (RunConfirmed's
// own `git status` read), but setupResidueChangedPaths' documented fallback
// names workingDirectory itself as an absolute path when residueUnknown is
// true -- rewriting that single entry relative to the worktree root keeps
// every path this function returns repository-relative, never leaking an
// absolute filesystem path to the customer.
func repositoryRelativeChangedPaths(dir string, changedPaths []string, residueUnknown bool) []string {
	if !residueUnknown || len(changedPaths) != 1 {
		return changedPaths
	}
	root := tstoolchain.WorktreeRoot(dir)
	rel, err := filepath.Rel(root, changedPaths[0])
	if err != nil {
		return changedPaths
	}
	return []string{rel}
}

// Bounds for setupResidueChangedPaths' read-only `git status` call: a small
// timeout and small output caps, since this is a status listing for a single
// working directory, not a whole-tree read.
const (
	setupResidueGitTimeout   = 10 * time.Second
	maxSetupResidueGitBytes  = 1 << 20 // 1 MiB
	maxSetupResidueGitStderr = 64 << 10
)

// parseSetupResidueStatusPaths extracts the path from each NUL-delimited
// `git status --porcelain -z` record ("XY<space><path>\0", XY being two
// status characters). Unlike the newline-delimited "--porcelain" format
// alone, -z never C-quotes or octal-escapes a path, so a path containing a
// space, non-ASCII byte, or literal quote character survives unmodified. A
// rename or copy record (status 'R' or 'C' in either column) emits the
// origin path as an additional NUL-delimited field immediately after the
// status/path field; that field is the path *before* the change, so it is
// consumed and discarded here -- only the resulting path is a "may have
// changed" location worth disclosing. Returns nil rather than an empty
// non-nil slice when there is nothing to report.
func parseSetupResidueStatusPaths(output []byte) []string {
	fields := bytes.Split(bytes.TrimRight(output, "\x00"), []byte{0})
	if len(fields) == 1 && len(fields[0]) == 0 {
		return nil
	}
	var paths []string
	for i := 0; i < len(fields); i++ {
		entry := fields[i]
		if len(entry) < 4 {
			continue
		}
		paths = append(paths, string(entry[3:]))
		if entry[0] == 'R' || entry[0] == 'C' || entry[1] == 'R' || entry[1] == 'C' {
			i++
		}
	}
	return paths
}

// runSetupResidueGit is the bounded git read behind setupResidueChangedPaths.
// Tests may replace it to observe exactly which git commands a setup run
// issues.
var runSetupResidueGit = func(dir string, args ...string) ([]byte, error) {
	return gitrepo.RunBytesBounded(dir, maxSetupResidueGitBytes, maxSetupResidueGitStderr, setupResidueGitTimeout, args...)
}

// setupResidueChangedPaths returns the untracked, modified, and gitignored
// paths (`--ignored`, since a package-manager install typically leaves a
// gitignored node_modules/ partially populated) that `git status --porcelain
// -z` reports under workingDirectory, for AC-SET-7's "identify files that
// may have changed" disclosure after a failed setup. The read is scoped to
// workingDirectory with a trailing `-- .` pathspec, so unrelated dirt
// elsewhere in a larger repository (workingDirectory can be a package
// directory inside a monorepo) is never reported. Returned paths are
// repository-root-relative, not workingDirectory-relative -- that is simply
// what `git status` reports, and a caller printing a path next to
// workingDirectory must account for the difference (a monorepo package at
// packages/app reports "packages/app/node_modules/", not "node_modules/").
// This only ever reads: it never invokes `git reset`/`git clean`/`git
// checkout` or any other command that could mutate workingDirectory.
//
// The returned bool is Outcome.ResidueUnknown (see its doc); it is true
// when the disclosure itself could not be produced -- workingDirectory is
// not inside a Git worktree, or the bounded git status call otherwise
// failed -- in which case the returned paths are a best-effort fallback
// (workingDirectory itself), not a real status read.
func setupResidueChangedPaths(workingDirectory string) ([]string, bool) {
	output, err := runSetupResidueGit(workingDirectory, "status", "--porcelain", "-z", "--ignored", "--", ".")
	if err != nil {
		return []string{workingDirectory}, true
	}
	return parseSetupResidueStatusPaths(output), false
}
