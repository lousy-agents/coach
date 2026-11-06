package projectconfig

import (
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/shellword"
)

// blobCheckFailure is the failure of the call that checks whether the policy's blob can be read. An object
// that is not in the repository is told in Coach's words, because git exits 1 and says nothing.
func blobCheckFailure(run gitrepo.Runner, dir, revision, repoPath string, err error) *gitFailure {
	failure := newGitFailure("whether it is committed at the analyzed revision", err)
	var absent *gitrepo.ObjectNotLocalError
	if errors.As(failure.err, &absent) {
		partial, _ := gitrepo.IsPartialClone(run, dir) // only chooses the wording; a read that cannot be told apart says the shorter thing
		failure.words = objectNotLocalWords(partial, revision, repoPath)
	}
	return failure
}

// objectNotLocalWords says the content is not here and that Coach does not fetch it. In a partial clone it
// names the command that does, when the path can be shown to a terminal and pasted as written.
func objectNotLocalWords(partialClone bool, revision, repoPath string) string {
	if !partialClone {
		return "its content is not in the local object store, and Coach does not fetch objects"
	}
	words := "its content is not in this partial clone, and Coach does not fetch objects"
	if command, ok := fetchCommand(revision, repoPath); ok {
		words += "; fetch it with " + command + " and run again"
	}
	return words
}

// fetchCommand reads the blob the way the check did, in the user's own shell where git may fetch it. It
// reports false for a path that is not one file inside the repository or cannot be shown safely.
func fetchCommand(revision, repoPath string) (string, bool) {
	if ValidatePath(repoPath) != nil || !shellword.Printable(repoPath) || !shellword.Printable(revision) {
		return "", false
	}
	return "git cat-file -e " + shellword.Quote(revision+":"+repoPath), true
}
