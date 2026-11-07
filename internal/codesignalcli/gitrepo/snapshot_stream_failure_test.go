package gitrepo

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// What the snapshot stream does when its consumer fails, when git says an object is missing, and when an
// object is empty.

// A consumer that fails stops the stream: git is killed and the visit's own error is returned. Without the
// kill, git blocks writing the rest of a response larger than the pipe, the read waits for the whole wall
// time, and the error is that the read took too long.
func TestReadObjectsKillsGitAndReturnsTheVisitsErrorWhenTheVisitFails(t *testing.T) {
	big := strings.Repeat("x", 256<<10) // past the 64 KiB of a pipe, so git blocks writing the rest
	repository, oids := committedBlobs(t, big, big, big)
	failure := errors.New("the visit failed")

	started := time.Now()
	_, err := readObjects(repository, oids, snapshotBudget{wall: time.Minute, object: 1 << 20}, func(string, []byte) error { return failure })
	elapsed := time.Since(started)

	if !errors.Is(err, failure) {
		t.Errorf("readObjects = %v, want the error of the visit, not a timeout", err)
	}
	if elapsed > 15*time.Second {
		t.Errorf("readObjects took %s to give up after a failed visit: git was left running", elapsed)
	}
}

func TestReadObjectsDoesNotVisitAnObjectGitSaysIsMissing(t *testing.T) {
	withStreamChild(t, func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "printf '0000 missing\\n1111 blob 2\\nhi\\n'")
	})
	var visited []string

	outcome, err := readObjects("", []string{"0000", "1111"}, snapshotBudget{wall: time.Minute, object: 1 << 20}, func(oid string, content []byte) error {
		visited = append(visited, oid+"="+string(content))
		return nil
	})

	if err != nil || len(outcome.missing) != 1 || outcome.missing[0] != "0000" {
		t.Errorf("readObjects = %+v, %v; want 0000 reported missing", outcome, err)
	}
	if len(visited) != 1 || visited[0] != "1111=hi" {
		t.Errorf("visited %q, want only the object git has: a missing object has no content to hand over", visited)
	}
}

func TestReadObjectsHandsOverAnEmptyObjectAsFoundAndEmpty(t *testing.T) {
	withStreamChild(t, func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "printf '0000 blob 0\\n\\n'")
	})
	var visited int
	var emptyContent bool

	outcome, err := readObjects("", []string{"0000"}, snapshotBudget{wall: time.Minute, object: 1 << 20}, func(_ string, content []byte) error {
		visited++
		emptyContent = len(content) == 0
		return nil
	})

	if err != nil || len(outcome.missing) != 0 || len(outcome.tooLarge) != 0 || visited != 1 || !emptyContent {
		t.Errorf("readObjects = %+v, %v after %d visits (empty: %v); want one visit of empty content, found", outcome, err, visited, emptyContent)
	}
}
