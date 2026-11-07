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
