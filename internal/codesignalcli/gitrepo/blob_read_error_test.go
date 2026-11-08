package gitrepo

import (
	"errors"
	"strings"
	"testing"
)

// The adapter reads a blob for whichever command the user ran. It must not name one: a scan reaches the same
// reads, and printed "--check-project" for a flag the user never passed.
func TestABlobReadFailureNamesNoCommandOfCoachs(t *testing.T) {
	cases := []struct {
		name  string
		fails string
	}{
		{"the revision cannot be verified", "cat-file -e"},
		{"the path cannot be resolved", "ls-tree"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertBlobReadFailureIsNeutral(t, c.fails)
		})
	}
}

func assertBlobReadFailureIsNeutral(t *testing.T, failing string) {
	t.Helper()
	boom := errors.New("exit status 128: fatal: bad object")
	run := func(_ string, args ...string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), failing) {
			return nil, boom
		}
		return nil, nil
	}

	_, _, err := BlobEntryAtRevision(run, "repo", "HEAD", "cfg/p.json")

	var operational *OperationalError
	if !errors.As(err, &operational) || strings.Contains(operational.Message, "coach codesignal") {
		t.Fatalf("BlobEntryAtRevision error = %v; want an OperationalError that names no command", err)
	}
	var read *BlobReadError
	if !errors.As(operational.Cause, &read) || read.Revision != "HEAD" || !errors.Is(read, boom) {
		t.Errorf("cause = %#v; want a *BlobReadError for HEAD that keeps git's own error", operational.Cause)
	}
}

func TestAnObjectAPartialCloneLacksIsAReadFailureThatKeepsItsType(t *testing.T) {
	skipIfGitCannotRefuseLazyFetch(t)
	clone, blob := partialClone(t, func(string) string { return "ssh://example.invalid/repo.git" })

	_, _, err := BlobEntryAtRevision(RunBytes, clone, "HEAD", "cfg/p.json")

	var operational *OperationalError
	if !errors.As(err, &operational) {
		t.Fatalf("BlobEntryAtRevision error = %v; want an OperationalError", err)
	}
	var read *BlobReadError
	var absent *ObjectNotLocalError
	if !errors.As(operational.Cause, &read) || read.Path != "cfg/p.json" || !errors.As(read, &absent) || absent.Object != blob {
		t.Errorf("cause = %#v; want a *BlobReadError for cfg/p.json whose cause is the missing blob %s", operational.Cause, blob)
	}
	if strings.Contains(operational.Message, "coach codesignal") || strings.Contains(operational.Message, "fetch") {
		t.Errorf("message %q words the failure for a command; that is for the layer that knows which", operational.Message)
	}
}

// The two forms say what failed in terms of the repository only: with a path, the path and the revision it was
// read at; without one, that the revision itself could not be verified. Neither names a command of Coach's.
func TestABlobReadErrorSaysWhichPathAndRevisionFailedAndWhy(t *testing.T) {
	cause := errors.New("fatal: bad object")
	cases := []struct {
		name string
		err  *BlobReadError
		want string
	}{
		{"a path that could not be read", &BlobReadError{Revision: "abc", Path: "cfg/p.json", Cause: cause}, `"cfg/p.json" could not be read at revision "abc": fatal: bad object`},
		{"a revision that could not be verified", &BlobReadError{Revision: "abc", Cause: cause}, `revision "abc" could not be verified: fatal: bad object`},
	}
	for _, c := range cases {
		if got := c.err.Error(); got != c.want {
			t.Errorf("%s: Error() = %q; want %q", c.name, got, c.want)
		}
	}
}
