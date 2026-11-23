package gitrepo

import (
	"errors"
	"fmt"
	"os/exec"
)

// ConfigHasNoValue is whether err is git config's exit 1, which is how it says that nothing matched: an
// unset key, or no key that matches. Every other failure (a damaged configuration, a time or output bound)
// leaves the question unanswered.
func ConfigHasNoValue(err error) bool {
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() == 1
}

// RefuseMissingObjects is nil unless the scan would read an object a partial clone lacks. An ordinary clone
// has every object; a repository that cannot be told apart from an ordinary one is refused, because reading it
// could run its remote's transport.
//
// A git that can be told not to fetch never does, so whether the repository is a partial clone only decides
// whether to look: a clone made without trees lacks the trees of HEAD, which nothing can be listed without, and
// a diff reads the merge base's version of every changed file, which the clone may lack. Both are refused, with
// the command that brings the objects in.
//
// On an older git, which ignores the instruction, the configuration decides nothing. What git takes for a
// promisor remote is more than the settings Coach reads (a remote with a partial clone filter is one, with
// promisor unset or false), and a read of a missing object from it fetches. So the revisions the scan reads are
// always asked about, with a probe that never fetches, and a revision that lacks objects is refused, whatever
// the configuration says; it only words the refusal. base is empty for a baseline, which reads HEAD, and
// otherwise the scan reads HEAD and the merge base of base and HEAD. A question the probe cannot answer is a
// refusal too, because a read that might fetch is not made on a guess.
func RefuseMissingObjects(run Runner, dir, base string) error {
	supported, version, err := askGitsVersion(run, dir)
	if err != nil {
		return versionUnknownError(err)
	}
	partial, partialErr := IsPartialClone(run, dir)
	if !supported {
		if partialErr != nil && BoundFailure(partialErr) != nil {
			return partialCloneUnanswered(partialErr)
		}
		if err := refuseWhenReadRevisionsLackObjects(run, dir, base, version); err != nil {
			return err
		}
		if partialErr != nil {
			return partialCloneUnanswered(partialErr)
		}
		return nil
	}
	if partialErr != nil {
		return partialCloneUnanswered(partialErr)
	}
	if !partial {
		return nil
	}
	if err := refuseWhenHeadLacksTrees(run, dir, base); err != nil {
		return err
	}
	return refuseWhenTheMergeBaseLacksObjects(run, dir, base)
}

// partialCloneUnanswered words a failure to tell whether the repository is a partial clone as the refusal it is. A
// git before 2.44 asks the question first too, as every git does: one that did not answer in time ends the guard
// there, naming it, rather than waiting again on the probes behind it. Any other failure is returned only after the
// probe, because on such a git a missing object is refused whatever the configuration says; it is then named as the
// question left unanswered, rather than as whichever later question happens to read the configuration (#737).
func partialCloneUnanswered(err error) error {
	if errors.Is(err, exec.ErrNotFound) {
		return gitNotFoundError()
	}
	return &OperationalError{Message: fmt.Sprintf("could not tell whether this repository is a partial clone (%s), so it is not read: a partial clone's read could run its remote's transport and write objects", TerminalSafe(err.Error())), Cause: err}
}
