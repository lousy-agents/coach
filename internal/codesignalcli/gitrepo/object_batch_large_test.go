package gitrepo

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// An object past the limit on one object can be taken as a stream: it is handed to the caller a chunk at a
// time and never allocated, and what the caller does not read of it is read past.

var oversizeBudget = snapshotBudget{wall: time.Minute, object: 50}

type largeCall struct {
	oid  string
	size int64
	body string
}

// largeTaker is a visitLarge that takes every object, reads all of it, and keeps what it was offered.
type largeTaker struct{ calls []largeCall }

func (t *largeTaker) take(oid string, size int64, body io.Reader) (bool, error) {
	content, err := io.ReadAll(body)
	t.calls = append(t.calls, largeCall{oid: oid, size: size, body: string(content)})
	return true, err
}

func TestReadObjectsStreamingHandsAnObjectPastTheLimitOverAsAStreamAndNotAsContent(t *testing.T) {
	big := strings.Repeat("x", 100)
	repository, oids := committedBlobs(t, big, "small")
	taker := &largeTaker{}
	var visited []string

	outcome, err := readObjectsStreaming(repository, oids, oversizeBudget, func(oid string, content []byte) error {
		visited = append(visited, oid+"="+string(content))
		return nil
	}, taker.take)

	if err != nil || len(outcome.tooLarge) != 0 {
		t.Fatalf("readObjectsStreaming = %+v, %v; want an object that was taken left out of the outcome", outcome, err)
	}
	if len(taker.calls) != 1 || taker.calls[0] != (largeCall{oid: oids[0], size: 100, body: big}) {
		t.Errorf("visitLarge was called with %+v, want the whole object as a stream of 100 bytes", taker.calls)
	}
	if len(visited) != 1 || visited[0] != oids[1]+"=small" {
		t.Errorf("visit saw %q, want only the object within the limit: the large one is never handed over as content", visited)
	}
}

func TestReadObjectsStreamingReadsPastWhatAVisitLargeDidNotRead(t *testing.T) {
	repository, oids := committedBlobs(t, strings.Repeat("x", 100), "small")
	var visited []string
	partial := func(_ string, _ int64, body io.Reader) (bool, error) {
		_, err := io.CopyN(io.Discard, body, 10)
		return true, err
	}

	_, err := readObjectsStreaming(repository, oids, oversizeBudget, func(oid string, content []byte) error {
		visited = append(visited, string(content))
		return nil
	}, partial)

	if err != nil || len(visited) != 1 || visited[0] != "small" {
		t.Errorf("readObjectsStreaming = %v, visited %q; want the next object intact after a partly read one", err, visited)
	}
}

func TestReadObjectsStreamingReportsAnObjectThatVisitLargeLeft(t *testing.T) {
	repository, oids := committedBlobs(t, strings.Repeat("x", 100), "small")
	var visited []string
	leave := func(string, int64, io.Reader) (bool, error) { return false, nil }

	outcome, err := readObjectsStreaming(repository, oids, oversizeBudget, func(oid string, content []byte) error {
		visited = append(visited, string(content))
		return nil
	}, leave)

	if err != nil || len(outcome.tooLarge) != 1 || outcome.tooLarge[0] != oids[0] || len(visited) != 1 || visited[0] != "small" {
		t.Errorf("readObjectsStreaming = %+v, %v, visited %q; want the left object reported and the next one intact", outcome, err, visited)
	}
}

func TestReadObjectsStreamingReturnsTheErrorOfVisitLargeAndStopsGit(t *testing.T) {
	repository, oids := committedBlobs(t, strings.Repeat("x", 100), "small")
	failure := errors.New("the write failed")

	_, err := readObjectsStreaming(repository, oids, oversizeBudget, func(string, []byte) error { return nil },
		func(string, int64, io.Reader) (bool, error) { return true, failure })

	if !errors.Is(err, failure) {
		t.Errorf("readObjectsStreaming = %v, want the error of visitLarge", err)
	}
}

// A stream that is cut inside an object is never taken for the object: the caller is told, and a file that
// stopped short is not written as if it were whole.
func TestAnObjectPastTheLimitThatIsCutShortEndsTheSnapshot(t *testing.T) {
	withStreamChild(t, func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "printf '1111 blob 100\\nonly ten b'")
	})
	withSnapshotLimits(t, oversizeBudget.object, snapshotTotalBytes)

	err := writeSnapshotWithin("", t.TempDir(), []snapshotEntry{{mode: "100644", path: "big.go", oid: "1111"}}, snapshotTotalBytes)

	if err == nil {
		t.Error("writeSnapshotWithin = nil, want the cut stream refused: ten bytes of a hundred are not the file")
	}
}
