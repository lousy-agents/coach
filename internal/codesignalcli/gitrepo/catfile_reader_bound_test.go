package gitrepo

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The reader of a --baseline scan is one long-lived git for the whole run, fed one request at a time. Each
// request is held to a wall time, and an object past the limit on one file is not allocated: it is discarded
// from the stream, which stays in step, and reported as too large for that file.

func withFileReadBudget(t *testing.T, replacement fileStreamBudget) {
	t.Helper()
	original := fileReadBudget
	t.Cleanup(func() { fileReadBudget = original })
	fileReadBudget = replacement
}

// gitThatStallsOnTheBatchReader is a git that answers every question but the --batch -Z reader, which it
// never answers.
func gitThatStallsOnTheBatchReader(t *testing.T) string {
	t.Helper()
	return gitThatStallsOnTheBatchReaderBy(t, "exec sleep 120")
}

// gitThatStallsOnTheBatchReaderBy is that git, stalling by the given command: "sleep 120 & wait" leaves a child
// of git that holds the pipe after git is killed.
func gitThatStallsOnTheBatchReaderBy(t *testing.T, stall string) string {
	t.Helper()
	real, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("find git: %v", err)
	}
	directory := t.TempDir()
	script := "#!/bin/sh\nfor argument in \"$@\"; do\n  if [ \"$argument\" = -Z ]; then " + stall + "; fi\ndone\nexec '" + real + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(directory, "git"), []byte(script), 0o755); err != nil {
		t.Fatalf("write the git that stalls: %v", err)
	}
	return directory
}

func TestRevisionFileReaderEndsARequestGitDoesNotAnswerAndStaysEnded(t *testing.T) {
	repository, _ := committedBlobs(t, "x")
	t.Setenv("PATH", gitThatStallsOnTheBatchReader(t)+string(os.PathListSeparator)+os.Getenv("PATH"))
	withFileReadBudget(t, fileStreamBudget{wall: 300 * time.Millisecond, object: 1 << 20})
	reader, err := NewRevisionFileReader(repository, "HEAD")
	if err != nil {
		t.Fatalf("NewRevisionFileReader: %v", err)
	}
	defer func() { _ = reader.Close() }()

	started := time.Now()
	_, first := reader.Next("fa.txt")
	_, second := reader.Next("fa.txt")
	_ = reader.Close()
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

func TestRevisionFileReaderDiscardsAnObjectPastTheLimitAndStaysInStep(t *testing.T) {
	repository, _ := committedBlobs(t, strings.Repeat("x", 100), "small")
	withFileReadBudget(t, fileStreamBudget{wall: time.Minute, object: 50})
	reader, err := NewRevisionFileReader(repository, "HEAD")
	if err != nil {
		t.Fatalf("NewRevisionFileReader: %v", err)
	}
	defer func() { _ = reader.Close() }()

	_, tooLarge := reader.Next("fa.txt")
	next, nextErr := reader.Next("fb.txt")

	var large *ObjectTooLargeError
	if !errors.As(tooLarge, &large) || large.Size != 100 || large.Limit != 50 || large.Path != "fa.txt" {
		t.Errorf("Next(fa.txt) error = %v, want an ObjectTooLargeError for fa.txt of 100 bytes against 50", tooLarge)
	}
	if nextErr != nil || string(next) != "small" {
		t.Errorf("Next(fb.txt) = %q, %v; want %q: the stream must stay in step after a discarded object", next, nextErr, "small")
	}
}

func TestRevisionFileReaderReadsAnObjectExactlyAtTheLimit(t *testing.T) {
	repository, _ := committedBlobs(t, strings.Repeat("y", 50))
	withFileReadBudget(t, fileStreamBudget{wall: time.Minute, object: 50})
	reader, err := NewRevisionFileReader(repository, "HEAD")
	if err != nil {
		t.Fatalf("NewRevisionFileReader: %v", err)
	}
	defer func() { _ = reader.Close() }()

	content, readErr := reader.Next("fa.txt")

	if readErr != nil || len(content) != 50 {
		t.Errorf("Next = %d bytes, %v; want the 50 bytes: an object at the limit is read", len(content), readErr)
	}
}

// A child of git that keeps the pipe open after git is killed must not keep the request waiting.
func TestRevisionFileReaderEndsARequestWhenAChildOfGitHoldsThePipe(t *testing.T) {
	repository, _ := committedBlobs(t, "x")
	t.Setenv("PATH", gitThatStallsOnTheBatchReaderBy(t, "sleep 20 & wait; exit")+string(os.PathListSeparator)+os.Getenv("PATH"))
	withFileReadBudget(t, fileStreamBudget{wall: 300 * time.Millisecond, object: 1 << 20})
	reader, err := NewRevisionFileReader(repository, "HEAD")
	if err != nil {
		t.Fatalf("NewRevisionFileReader: %v", err)
	}
	defer func() { _ = reader.Close() }()

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
