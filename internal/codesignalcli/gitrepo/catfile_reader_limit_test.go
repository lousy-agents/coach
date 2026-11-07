package gitrepo

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// An object past the limit on one file is discarded from the stream, which stays in step, and reported as
// too large for that file.

func TestRevisionFileReaderDiscardsAnObjectPastTheLimitAndStaysInStep(t *testing.T) {
	repository, _ := committedBlobs(t, strings.Repeat("x", 100), "small")
	withFileReadBudget(t, fileStreamBudget{wall: time.Minute, object: 50})
	reader, err := NewRevisionFileReader(repository, "HEAD")
	if err != nil {
		t.Fatalf("NewRevisionFileReader: %v", err)
	}
	defer reader.Close()

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
	defer reader.Close()

	content, readErr := reader.Next("fa.txt")

	if readErr != nil || len(content) != 50 {
		t.Errorf("Next = %d bytes, %v; want the 50 bytes: an object at the limit is read", len(content), readErr)
	}
}
