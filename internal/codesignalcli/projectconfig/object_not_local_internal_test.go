package projectconfig

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// runnerOfRepository answers the one question the wording depends on, which remote is a promisor, as git
// does: the setting for a partial clone, and exit 1 for a repository with none. Any other question is
// one this wording must not ask.
func runnerOfRepository(partialClone bool) gitrepo.Runner {
	return func(_ string, args ...string) ([]byte, error) {
		if len(args) == 0 || args[0] != "config" {
			return nil, errors.New("unexpected git question: " + strings.Join(args, " "))
		}
		if partialClone && slices.Contains(args, "--get-regexp") {
			return []byte("remote.origin.promisor true\n"), nil
		}
		return nil, gitSaysUnset
	}
}

func TestAnObjectThatIsNotLocalIsToldInCoachsWordsAndNotGitsBareExitStatus(t *testing.T) {
	absent := &gitrepo.OperationalError{Message: "m", Cause: &gitrepo.ObjectNotLocalError{Object: "abc"}}
	cases := []struct {
		name         string
		partialClone bool
		path         string
		want         []string
		avoid        []string
	}{
		{"a partial clone and a path that can be shown", true, "cfg/p.json",
			[]string{"could not check whether it is committed at the analyzed revision: its content is not in this partial clone, and Coach does not fetch objects; fetch it with git cat-file -e 'abc123:cfg/p.json', or the revision's files with git rev-list --objects --missing=print --no-walk abc123 | sed -n 's/^?//p' | git fetch origin --no-tags --no-write-fetch-head --filter=blob:none --stdin, and run again"}, []string{"git said"}},
		{"a partial clone and a name with a control character", true, "cfg/esc\x1b[31m.json",
			[]string{"its content is not in this partial clone"}, []string{"git cat-file", "fetch it with", "git said"}},
		{"a partial clone and a path outside the repository", true, "../p.json",
			[]string{"its content is not in this partial clone"}, []string{"git cat-file", "fetch it with"}},
		{"a repository that is not a partial clone", false, "cfg/p.json",
			[]string{"its content is not in the local object store, and Coach does not fetch objects"}, []string{"git cat-file", "fetch it with", "partial clone", "git said"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertNotLocalWords(t, blobCheckFailure(runnerOfRepository(c.partialClone), "repo", "abc123", c.path, absent).Error(), c.want, c.avoid)
		})
	}
}

func assertNotLocalWords(t *testing.T, got string, want, avoid []string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("failure %q; want it to contain %q", got, w)
		}
	}
	for _, a := range avoid {
		if strings.Contains(got, a) {
			t.Errorf("failure %q; want it not to contain %q", got, a)
		}
	}
}

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

func TestAnyOtherFailureOfTheBlobCheckStillQuotesWhatGitSaid(t *testing.T) {
	got := blobCheckFailure(runnerOfRepository(true), "repo", "abc123", "cfg/p.json", errors.New("boom")).Error()

	if !strings.Contains(got, `(git said "boom")`) || strings.Contains(got, "partial clone") {
		t.Errorf("failure %q; want git's own words and no claim about a partial clone", got)
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
