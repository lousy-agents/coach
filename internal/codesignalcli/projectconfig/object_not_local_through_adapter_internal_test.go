package projectconfig

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// A fetch command is built from the remote's name and the revision, so a name that cannot be pasted safely
// leaves the words without one, and without the separator that would have introduced it.
func TestNoFetchCommandIsOfferedForARemoteWhoseNameCannotBePastedSafely(t *testing.T) {
	run := func(_ string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "config" && slices.Contains(args, "--get-regexp") {
			return []byte("remote.origin\x1b[31m.promisor true\n"), nil
		}
		return nil, gitSaysUnset
	}

	got := blobCheckFailure(run, "repo", "abc123", "cfg/p.json", &gitrepo.OperationalError{Message: "m", Cause: &gitrepo.ObjectNotLocalError{Object: "abc"}}).Error()

	if !strings.HasSuffix(got, "Coach does not fetch objects") || strings.Contains(got, "fetch it with") || strings.Contains(got, "git fetch") {
		t.Errorf("failure %q; want the plain statement, with no command and no trailing separator", got)
	}
}

// The adapter returns the failure typed, with the object a partial clone lacks inside a blob-read error. The
// question that asks "whether it is committed" words it in its own framing, so it reads through that wrapper.
func TestTheCommitQuestionReadsAMissingObjectThroughTheAdaptersBlobReadError(t *testing.T) {
	read := &gitrepo.BlobReadError{Revision: "abc123", Path: "cfg/p.json", Cause: &gitrepo.ObjectNotLocalError{Object: "abc"}}
	adapterError := &gitrepo.OperationalError{Message: read.Error(), Cause: read}

	failure := blobCheckFailure(runnerOfRepository(true), "repo", "abc123", "cfg/p.json", adapterError)

	var absent *gitrepo.ObjectNotLocalError
	if _, stillWrapped := errors.Unwrap(failure).(*gitrepo.BlobReadError); stillWrapped || !errors.As(failure, &absent) {
		t.Errorf("failure %v leads to %#v; want git's own error as the cause, not the adapter's wrapper, which the command line would word again", failure, errors.Unwrap(failure))
	}
	if got := failure.Error(); !strings.Contains(got, "could not check whether it is committed") || !strings.Contains(got, "its content is not in this partial clone") || strings.Contains(got, "could not be read at revision") {
		t.Errorf("failure %q; want the question's own framing around the one sentence for a missing object", got)
	}
}
