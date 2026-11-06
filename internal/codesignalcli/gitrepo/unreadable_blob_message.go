package gitrepo

import (
	"errors"
	"fmt"
)

// unreadableBlobMessage words a blob that is named by the tree and could not be read. An object the repository
// lacks is told as that, with the way to fetch it in a partial clone, because git's own exit status says
// nothing; any other failure keeps what git said.
func unreadableBlobMessage(run Runner, dir, revision, repoPath string, cause error) string {
	var absent *ObjectNotLocalError
	if !errors.As(cause, &absent) {
		return fmt.Sprintf("coach codesignal --check-project: %q could not be read at revision %q: %s", repoPath, revision, cause)
	}
	message := fmt.Sprintf("coach codesignal --check-project: %q could not be read at revision %q: its content is not in the local object store, and Coach does not fetch objects", repoPath, revision)
	if advice := FetchAdvice(run, dir, revision, repoPath); advice != "" {
		message += "; " + advice
	}
	return message
}
