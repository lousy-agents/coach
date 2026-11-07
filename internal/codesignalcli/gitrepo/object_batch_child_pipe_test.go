package gitrepo

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// The wall bound of a snapshot read ends the read, not only the one process the context can kill. A child
// of git (a filter, a helper) keeps stdout and stderr open after git is killed, and a read that waits for
// the end of the stream waits for the child.

func TestReadObjectsEndsAtItsBoundWhenAChildOfGitKeepsThePipesOpen(t *testing.T) {
	// git starts a child that holds the pipes and waits for it: killing git leaves the child
	withStreamChild(t, func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "sleep 30 & wait; exit")
	})
	const wall = 500 * time.Millisecond

	started := time.Now()
	_, err := readObjects("", []string{"0000"}, snapshotBudget{wall: wall, object: 1 << 20}, func(string, []byte) error { return nil })
	elapsed := time.Since(started)

	if err == nil || !strings.Contains(err.Error(), "took longer than") {
		t.Errorf("readObjects = %v; want an error that says the read took longer than its bound", err)
	}
	if ceiling := wall + 5*time.Second; elapsed > ceiling {
		t.Errorf("readObjects took %s against a bound of %s: the read outlived its bound by waiting for a child of git", elapsed, wall)
	}
}
