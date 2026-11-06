package gitrepo

import (
	"errors"
	"testing"
)

// A git that does not answer is not a git that answered no. The reads that turn a failed answer into a
// message of their own (this is not a worktree, HEAD is not readable) must not report a read that ran out
// of time as one of those. The partial-clone guard, which reads through its own bounded runner, has the
// same rule in lazy_fetch_guard_test.go.

const timedOutMessage = "coach codesignal: git rev-parse: git execution timed out after 30s"

func timedOutRead() error {
	return askedAbout([]string{"rev-parse", "HEAD"}, &BoundError{Kind: BoundTimeout, message: "git execution timed out after 30s"})
}

type boundFailureCase struct {
	name         string
	err          error
	wantAny      bool
	wantTimedOut bool
}

func (c boundFailureCase) run(t *testing.T) {
	if got := BoundFailure(c.err) != nil; got != c.wantAny {
		t.Errorf("BoundFailure(%v) != nil = %v, want %v", c.err, got, c.wantAny)
	}
	if got := TimedOut(c.err) != nil; got != c.wantTimedOut {
		t.Errorf("TimedOut(%v) != nil = %v, want %v", c.err, got, c.wantTimedOut)
	}
}

func TestBoundFailureTellsAReadThatRanOutFromOneThatAnsweredNo(t *testing.T) {
	outputOverrun := askedAbout([]string{"show", "x:y"}, &BoundError{Kind: BoundStdout, message: "git stdout exceeded 1-byte budget"})
	cases := []boundFailureCase{
		{"a read that ran out of time", timedOutRead(), true, true},
		{"a read that printed more than its budget", outputOverrun, true, false},
		{"git answering with a failure", errors.New("exit status 128: fatal: not a git repository"), false, false},
		{"no error", nil, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, c.run)
	}
}

func TestBoundFailureNamesTheGitQuestionThatDidNotAnswer(t *testing.T) {
	failure := BoundFailure(timedOutRead())

	if failure == nil || failure.Error() != timedOutMessage {
		t.Errorf("BoundFailure message = %v, want %q", failure, timedOutMessage)
	}
}
