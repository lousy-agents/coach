package gitrepo

import (
	"bytes"
	"errors"
	"testing"
	"testing/iotest"
)

const overrunTestBudget = 8

type overrunCase struct {
	name         string
	streamLength int
	wantEnded    bool
	wantRead     int
}

func overrunCases() []overrunCase {
	return []overrunCase{
		{"an empty stream", 0, false, 0},
		{"a stream one byte under the budget", overrunTestBudget - 1, false, overrunTestBudget - 1},
		{"a stream of exactly the budget", overrunTestBudget, false, overrunTestBudget},
		{"a stream one byte over the budget", overrunTestBudget + 1, true, overrunTestBudget + 1},
		{"a stream far over the budget", 10 * overrunTestBudget, true, overrunTestBudget + 1},
	}
}

// readGitPipeUntilOverrun ends git the moment a stream proves to be over its budget, and not before:
// a stream of exactly the budget is within it, and git has nothing left to be stopped from.
func TestReadGitPipeUntilOverrunEndsGitOnlyOncePastTheBudget(t *testing.T) {
	for _, c := range overrunCases() {
		t.Run(c.name, func(t *testing.T) { assertOverrunOutcome(t, c) })
	}
}

func assertOverrunOutcome(t *testing.T, c overrunCase) {
	t.Helper()
	ended := 0

	result := readGitPipeUntilOverrun(bytes.NewReader(make([]byte, c.streamLength)), overrunTestBudget, func() { ended++ })

	if result.err != nil {
		t.Fatalf("read error = %v, want none", result.err)
	}
	if (ended > 0) != c.wantEnded || ended > 1 {
		t.Errorf("git ended %d times, want %s for a %d-byte stream with a %d-byte budget", ended, endedWanted(c.wantEnded), c.streamLength, overrunTestBudget)
	}
	if len(result.data) != c.wantRead {
		t.Errorf("read %d bytes, want %d: one byte past the budget proves the overrun, and no more is kept", len(result.data), c.wantRead)
	}
}

func endedWanted(wantEnded bool) string {
	if wantEnded {
		return "exactly once"
	}
	return "never"
}

func TestReadGitPipeUntilOverrunReportsAReadFailureWithoutEndingGit(t *testing.T) {
	readFailure := errors.New("pipe closed")
	ended := false

	result := readGitPipeUntilOverrun(iotest.ErrReader(readFailure), overrunTestBudget, func() { ended = true })

	if !errors.Is(result.err, readFailure) {
		t.Errorf("read error = %v, want %v", result.err, readFailure)
	}
	if ended {
		t.Error("git was ended for a read that failed before the stream showed an overrun")
	}
}
