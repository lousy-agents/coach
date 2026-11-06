// Package gitrepo is the git process adapter behind coach codesignal:
// revision resolution, changed and tracked file selection, bounded and
// batched object reads, revision archives, and worktree status.
package gitrepo

import (
	"errors"
	"fmt"
)

func Run(dir string, args ...string) (string, error) {
	output, err := RunBytes(dir, args...)
	return string(output), err
}

// RunBytes is one of the scan's own git reads, held to the budget of its subcommand (see git_budgets.go):
// a git that does not answer in time, or that prints more than the read may hold, is an error that names
// the git question. It is a *BoundError beneath the wrapping, so a caller can tell it from a git that
// answered with a failure.
func RunBytes(dir string, args ...string) ([]byte, error) {
	limit := budgetFor(args)
	output, err := RunBytesBounded(dir, limit.stdout, limit.stderr, limit.timeout, args...)
	return output, askedAbout(args, err)
}

// askedAbout puts the git subcommand in front of a bound error, so that "git execution timed out" says
// which question did not answer. Any other error is returned as it is.
func askedAbout(args []string, err error) error {
	var bound *BoundError
	if len(args) == 0 || !errors.As(err, &bound) {
		return err
	}
	return fmt.Errorf("git %s: %w", args[0], err)
}

// BoundFailure is the *OperationalError for a git read that ran out of time or output, and nil for any
// other error. It returns an error and not the pointer, so that "no bound failure" is a nil error and not
// a non-nil error holding a nil pointer. A caller that turns a failed read into a message of its own (this
// is not a worktree, HEAD is not readable) uses it first, so that a git that never answered is not
// reported as one that answered no.
func BoundFailure(err error) error {
	var bound *BoundError
	if !errors.As(err, &bound) {
		return nil
	}
	return boundOperationalError(err)
}

// TimedOut is BoundFailure for a git that did not answer in time only. A per-file read that prints more
// than its budget is that file's problem, and the scan goes on without it; a git that does not answer
// will not answer the next file either, so the scan stops instead of waiting out its budget once per file.
func TimedOut(err error) error {
	var bound *BoundError
	if !errors.As(err, &bound) || bound.Kind != BoundTimeout {
		return nil
	}
	return boundOperationalError(err)
}

func boundOperationalError(err error) error {
	return &OperationalError{Message: "coach codesignal: " + err.Error(), Cause: err}
}
