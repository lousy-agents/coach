package gitrepo

import (
	"errors"
	"strings"
	"testing"
)

// A check of a blob that ran out of time is not an object that is missing. The bound is kept as the cause of the
// failure to read it, so a caller that classifies bounds (BoundFailure, TimedOut) sees a git that did not answer.
func TestABlobCheckThatTimedOutKeepsItsBoundForTheCaller(t *testing.T) {
	const blob = "0123456789012345678901234567890123456789"
	run := func(_ string, args ...string) ([]byte, error) {
		switch {
		case args[0] == "cat-file" && args[len(args)-1] == blob:
			return nil, &BoundError{Kind: BoundTimeout}
		case args[0] == "cat-file":
			return nil, nil
		case args[0] == "ls-tree":
			return []byte("100644 blob " + blob + "\tcfg/p.json\x00"), nil
		}
		return nil, errors.New("unexpected question " + strings.Join(args, " "))
	}

	_, _, err := BlobEntryAtRevision(run, "repo", "HEAD", "cfg/p.json")

	var operational *OperationalError
	var read *BlobReadError
	var bound *BoundError
	if !errors.As(err, &operational) || !errors.As(operational.Cause, &read) || !errors.As(read.Cause, &bound) || bound.Kind != BoundTimeout {
		t.Errorf("BlobEntryAtRevision error = %#v; want git's own failure (the timeout) kept as the cause of the cause", err)
	}
}
