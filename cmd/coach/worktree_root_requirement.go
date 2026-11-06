package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

const worktreeRootRequirement = "coach: --worktree must run from the repository root"

// The generic usage line is left out of this error: it says a base or baseline
// is required, which a bare --worktree does not need, and it does not help the
// reader find the directory to run from.

// rejectWorktreeOutsideRepositoryRoot enforces the root-only contract of
// --worktree: working-tree paths are repository-relative while a scan started
// below the root resolves them against that directory, so the two would
// disagree without any signal. A directory that is not a repository is left to
// the scan's own error. The error ends with the command to run instead, quoted
// so that it can be pasted into a shell.
func rejectWorktreeOutsideRepositoryRoot(dir string, args []string, f codesignalFlags, stderr *os.File) bool {
	if !f.worktree {
		return false
	}
	root, err := gitrepo.RepositoryRoot(dir)
	if err != nil {
		return false
	}
	if sameDirectory(dir, root) {
		return false
	}
	fmt.Fprintf(stderr, "%s; run: cd %s && coach codesignal%s\n", worktreeRootRequirement, quoteForShell(root), argumentsForShell(args))
	return true
}

// argumentsForShell renders each argument after a space, quoted only when the
// shell would otherwise split or expand it.
func argumentsForShell(args []string) string {
	var rendered strings.Builder
	for _, arg := range args {
		rendered.WriteByte(' ')
		if plainShellWord.MatchString(arg) {
			rendered.WriteString(arg)
		} else {
			rendered.WriteString(quoteForShell(arg))
		}
	}
	return rendered.String()
}

// plainShellWord is a word the shell leaves as typed. A tilde is plain except at
// the start of the word, where the shell expands it to a home directory.
var plainShellWord = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-][A-Za-z0-9_@%+=:,./~-]*$`)

// quoteForShell wraps value in single quotes, which no shell expands, closing
// and reopening them around any single quote the value holds.
func quoteForShell(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// sameDirectory compares by file identity, not by spelling: the shell can hand
// the process a path whose letter case or links differ from the root git
// reports, and a string comparison would refuse the root itself.
func sameDirectory(a, b string) bool {
	first, err := os.Stat(a)
	if err != nil {
		return false
	}
	second, err := os.Stat(b)
	return err == nil && os.SameFile(first, second)
}
