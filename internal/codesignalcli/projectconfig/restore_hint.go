package projectconfig

import "github.com/lousy-agents/coach/internal/codesignalcli/shellword"

// RestoreCommand is the command a user can paste into a POSIX shell, from any
// directory, to bring a committed policy back into the index and the working
// tree. :(top) makes the pathspec relative to the repository root, which is
// what repoPath is, and literal stops a name holding * ? or [ from being read as
// a glob. It reports false for a name that cannot be shown safely on a
// terminal, and for a path that does not name one file inside the repository:
// an empty pathspec would restore every tracked file. The caller then names no
// command.
func RestoreCommand(revision, repoPath string) (string, bool) {
	if ValidatePath(repoPath) != nil || !shellword.Printable(repoPath) || !shellword.Printable(revision) {
		return "", false
	}
	return "git restore --source=" + shellword.Word(revision) + " --staged --worktree -- " + shellword.Quote(":(top,literal)"+repoPath), true
}

// RestoreAdvice is the sentence that tells a user how to bring a committed
// policy back, ending on the command so the line pastes as written. A name that
// cannot be printed gets the reason in place of the command.
func RestoreAdvice(revision, repoPath string) string {
	if command, ok := RestoreCommand(revision, repoPath); ok {
		return "restore it with " + command
	}
	return "restore it from git at the analyzed revision (the name has a character that is not printed here)"
}
