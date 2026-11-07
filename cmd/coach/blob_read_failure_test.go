package main

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// remoteAnswering is a runner that answers the one question the wording asks, which remote is a promisor.
func remoteAnswering(partialClone bool) gitrepo.Runner {
	return func(_ string, args ...string) ([]byte, error) {
		if partialClone && len(args) > 2 && args[2] == "--get-regexp" {
			return []byte("remote.origin.promisor true\n"), nil
		}
		return nil, exec.Command("sh", "-c", "exit 1").Run()
	}
}

func readFailure(cause error) error {
	read := &gitrepo.BlobReadError{Revision: "abc123", Path: "cfg/p.json", Cause: cause}
	return &gitrepo.OperationalError{Message: read.Error(), Cause: read}
}

// The same condition, a policy blob a partial clone lacks, is worded once. The mode only chooses the prefix.
func TestABlobAPartialCloneLacksIsWordedOnceAndPrefixedForTheMode(t *testing.T) {
	absent := readFailure(&gitrepo.ObjectNotLocalError{Object: "9e26dfee"})
	tail := `"cfg/p.json" could not be read at revision "abc123": its content is not in this partial clone, and Coach does not fetch objects; fetch it with git cat-file -e 'abc123:cfg/p.json', or the revision's files with git rev-list --objects --missing=print --no-walk abc123 | sed -n 's/^?//p' | git fetch origin --no-tags --no-write-fetch-head --filter=blob:none --stdin, and run again`
	for _, prefix := range []string{"coach codesignal --check-project: ", "coach codesignal: "} {
		t.Run(prefix, func(t *testing.T) {
			got := wordBlobReadFailure(remoteAnswering(true), "repo", absent, prefix)

			if got == nil || got.Error() != prefix+tail {
				t.Errorf("worded = %v; want %q", got, prefix+tail)
			}
		})
	}
}

func TestABlobAnOrdinaryRepositoryLacksIsToldWithoutThePartialCloneAdvice(t *testing.T) {
	got := wordBlobReadFailure(remoteAnswering(false), "repo", readFailure(&gitrepo.ObjectNotLocalError{Object: "9e26dfee"}), "coach codesignal: ")

	want := `coach codesignal: "cfg/p.json" could not be read at revision "abc123": its content is not in the local object store, and Coach does not fetch objects`
	if got == nil || got.Error() != want {
		t.Errorf("worded = %v; want %q", got, want)
	}
}

func TestAnyOtherFailureToReadABlobKeepsItsMessageAndGetsTheModePrefix(t *testing.T) {
	boom := errors.New("exit status 128: fatal: bad object")
	original := readFailure(boom)

	got := wordBlobReadFailure(remoteAnswering(true), "repo", original, "coach codesignal --check-project: ")

	var operational *gitrepo.OperationalError
	if !errors.As(got, &operational) || operational.Message != "coach codesignal --check-project: "+original.Error() || !errors.Is(operational.Cause, boom) {
		t.Errorf("worded = %#v; want the neutral message behind the prefix, with git's error still the cause", got)
	}
}

func TestAnErrorThatIsNotABlobReadFailureIsLeftAsItIs(t *testing.T) {
	cases := map[string]error{
		"a plain error":                 errors.New("boom"),
		"an operational error":          &gitrepo.OperationalError{Message: "reading source snapshot: 2 objects are not in the local object store"},
		"an operational error of a git": &gitrepo.OperationalError{Message: "x", Cause: errors.New("y")},
	}
	for name, original := range cases {
		t.Run(name, func(t *testing.T) {
			if got := wordBlobReadFailure(remoteAnswering(true), "repo", original, "coach codesignal: "); got != original || strings.Contains(got.Error(), "coach codesignal:") {
				t.Errorf("worded = %v; want the error itself, untouched", got)
			}
		})
	}
}

// A revision that could not be verified has no path to fetch, so even an absent object is worded as the
// repository's own failure, behind the mode prefix.
func TestARevisionThatCouldNotBeVerifiedIsNeverWordedAsAPolicyBlobToFetch(t *testing.T) {
	read := &gitrepo.BlobReadError{Revision: "abc123", Cause: &gitrepo.ObjectNotLocalError{Object: "9e26dfee"}}
	original := &gitrepo.OperationalError{Message: read.Error(), Cause: read}

	got := wordBlobReadFailure(remoteAnswering(true), "repo", original, "coach codesignal: ")

	if got == nil || got.Error() != "coach codesignal: "+read.Error() || strings.Contains(got.Error(), "fetch it with") {
		t.Errorf("worded = %v; want the neutral message behind the prefix and no fetch advice", got)
	}
}
