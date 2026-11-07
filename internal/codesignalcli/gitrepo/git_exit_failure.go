package gitrepo

import (
	"errors"
	"os/exec"
)

// gitExitFailure is a git that ran to the end and exited with a failure, with what it said on stderr. It reads
// as the same text it always did, and it is deliberately not an *exec.ExitError to errors.As: a silent exit 1 is
// how git config and cat-file -e say "no", and an exit 1 that printed a message is an error, not that answer.
type gitExitFailure struct{ message string }

func (f *gitExitFailure) Error() string { return f.message }

// gitRanAndFailed is whether err is a git that finished and failed, silently or with a message. A git that ran
// out of time, overran its output, or could not be started has not answered, and is not this.
func gitRanAndFailed(err error) bool {
	var exit *exec.ExitError
	var failure *gitExitFailure
	return errors.As(err, &exit) || errors.As(err, &failure)
}
