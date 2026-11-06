package projectconfig

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/shellword"
)

// restoreCommand is the command a user can paste into a POSIX shell, from any
// directory, to bring a committed policy back into the index and the working
// tree. :(top) makes the pathspec relative to the repository root, which is
// what repoPath is, and literal stops a name holding * ? or [ from being read as
// a glob. It reports false for a name that cannot be shown safely on a
// terminal, and for a path that does not name one file inside the repository:
// an empty pathspec would restore every tracked file. The caller then names no
// command.
func restoreCommand(revision, repoPath string) (string, bool) {
	if ValidatePath(repoPath) != nil || !shellword.Printable(repoPath) || !shellword.Printable(revision) {
		return "", false
	}
	return "git restore --source=" + shellword.Word(revision) + " --staged --worktree -- " + shellword.Quote(":(top,literal)"+repoPath), true
}

// RestoreAdvice is the sentence that tells a user how to bring a committed
// policy back, ending on the command so the line pastes as written. A name that
// cannot be printed gets the reason in place of the command.
func RestoreAdvice(revision, repoPath string) string {
	if command, ok := restoreCommand(revision, repoPath); ok {
		return "restore it with " + command
	}
	return "restore it from git at the analyzed revision (the name has a character that is not printed here)"
}

// RestoreFromIndexAdvice is how to bring back a file the index still holds: git
// restore without a source restores the working tree from the index and leaves
// the index alone.
func RestoreFromIndexAdvice(repoPath string) string {
	if ValidatePath(repoPath) == nil && shellword.Printable(repoPath) {
		return "restore it with git restore -- " + shellword.Quote(":(top,literal)"+repoPath)
	}
	return "restore it from the index (the name has a character that is not printed here)"
}

// clearSkipWorktreeAdvice is the command that clears a skip-worktree bit. It
// takes a path relative to the repository root, not a pathspec.
func clearSkipWorktreeAdvice(repoPath string) string {
	if ValidatePath(repoPath) == nil && shellword.Printable(repoPath) {
		return "clear the mark with git update-index --no-skip-worktree -- " + shellword.Quote(repoPath) + " from the repository root"
	}
	return "clear the mark with git update-index --no-skip-worktree (the name has a character that is not printed here)"
}

// RestoreFraming is how the sentence that says a committed policy can be brought back is joined to
// what comes before it: as an offer that continues a message, or as a clause of the refusal of
// compiler setup, which also names the next action it is.
type RestoreFraming struct {
	opening string
	action  string
}

// AsAnOffer continues the message of a missing policy with a sentence.
var AsAnOffer = RestoreFraming{opening: ". It"}

// AsARefusal continues the refusal of compiler setup with a clause that names action.
func AsARefusal(action string) RestoreFraming {
	return RestoreFraming{opening: "; it", action: action}
}

const emptyEntryReason = ", and the index holds only an empty entry for it with no content staged, so restoring brings back the committed copy: "

// RestoreSentence says that the policy is committed at the analyzed revision and how to bring it
// back, ending on the command so the line pastes as written. It is the one place the sentence is
// built, for the offer and for the refusal, and the only one that knows a restore over an empty
// index entry says why it loses nothing.
func RestoreSentence(remedy projectreadiness.PolicyRemedy, revision, repoPath string, framing RestoreFraming) string {
	sentence := framing.opening + " is committed at the analyzed revision"
	if framing.action != "" {
		sentence += " (" + framing.action + ")"
	}
	return sentence + restoreConnective(remedy, framing) + RestoreAdvice(revision, repoPath)
}

// restoreConnective joins what is said about the committed copy to the command.
func restoreConnective(remedy projectreadiness.PolicyRemedy, framing RestoreFraming) string {
	switch {
	case remedy == projectreadiness.RemedyRestoreOverEmptyEntry:
		return emptyEntryReason
	case framing.action != "":
		return ", so "
	}
	return ": "
}
