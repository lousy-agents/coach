package gitrepo

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// The reader of a --baseline scan is one long-lived git for the whole run, fed one request at a time. Each
// request is held to a wall time. The git here is a fake child that is the whole of git for the test.

func readerOver(t *testing.T, child func(ctx context.Context) *exec.Cmd) *RevisionFileReader {
	t.Helper()
	reader, err := newRevisionFileReader(streamChild(t, child), fileStreamBudget{wall: 300 * time.Millisecond, object: 1 << 20}, "", "HEAD")
	if err != nil {
		t.Fatalf("newRevisionFileReader: %v", err)
	}
	t.Cleanup(reader.Close)
	return reader
}

func TestRevisionFileReaderEndsARequestGitDoesNotAnswerAndStaysEnded(t *testing.T) {
	reader := readerOver(t, func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "sleep", "120") })

	started := time.Now()
	_, first := reader.Next("fa.txt")
	_, second := reader.Next("fa.txt")
	reader.Close()
	elapsed := time.Since(started)

	var bound *BoundError
	if !errors.As(first, &bound) || bound.Kind != BoundTimeout || !strings.Contains(first.Error(), "git cat-file") {
		t.Errorf("first Next error = %v, want a timeout BoundError that names git cat-file", first)
	}
	if !errors.As(second, &bound) || bound.Kind != BoundTimeout {
		t.Errorf("second Next error = %v, want the same timeout (a git that did not answer will not answer the next request)", second)
	}
	if elapsed > 5*time.Second {
		t.Errorf("two requests and the close took %s against a wall time of 300ms: the stalled git must be killed", elapsed)
	}
}

// A child of git that keeps the pipe open after git is killed must not keep the request waiting.
func TestRevisionFileReaderEndsARequestWhenAChildOfGitHoldsThePipe(t *testing.T) {
	reader := readerOver(t, func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "sleep 20 & wait; exit")
	})

	started := time.Now()
	_, readErr := reader.Next("fa.txt")
	elapsed := time.Since(started)

	var bound *BoundError
	if !errors.As(readErr, &bound) || bound.Kind != BoundTimeout {
		t.Errorf("Next error = %v, want a timeout BoundError", readErr)
	}
	if elapsed > 5*time.Second {
		t.Errorf("the request took %s to end against a wall time of 300ms: it waited for a child of git", elapsed)
	}
}
