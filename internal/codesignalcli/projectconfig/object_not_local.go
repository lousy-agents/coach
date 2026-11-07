package projectconfig

import (
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// blobCheckFailure is the failure of the call that checks whether the policy's blob can be read. An object
// that is not in the repository is told in Coach's words, because git exits 1 and says nothing.
func blobCheckFailure(run gitrepo.Runner, dir, revision, repoPath string, err error) *gitFailure {
	failure := newGitFailure("whether it is committed at the analyzed revision", err)
	var absent *gitrepo.ObjectNotLocalError
	if errors.As(failure.err, &absent) {
		failure.words = ObjectNotLocalWords(run, dir, revision, repoPath)
	}
	return failure
}

// ObjectNotLocalWords is the one sentence for a policy blob that is not in the repository. It says the content is not here and that Coach does not fetch it. In a partial clone it
// adds the way to fetch it. The single-object command is offered only for a path that names one file inside
// the repository, since the batch fetches every file of the revision and needs no path.
func ObjectNotLocalWords(run gitrepo.Runner, dir, revision, repoPath string) string {
	if _, partialClone := gitrepo.PromisorRemote(run, dir); !partialClone {
		return "its content is not in the local object store, and Coach does not fetch objects"
	}
	path := repoPath
	if ValidatePath(repoPath) != nil {
		path = ""
	}
	words := "its content is not in this partial clone, and Coach does not fetch objects"
	if advice := gitrepo.FetchAdvice(run, dir, revision, path); advice != "" {
		words += "; " + advice
	}
	return words
}
