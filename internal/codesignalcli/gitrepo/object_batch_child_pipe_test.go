package gitrepo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The wall bound of a snapshot read ends the read, not only the one process the context can kill. A child
// of git (a filter, a helper) keeps stdout and stderr open after git is killed, and a read that waits for
// the end of the stream waits for the child.

func TestReadObjectsEndsAtItsBoundWhenAChildOfGitKeepsThePipesOpen(t *testing.T) {
	repository, oids := committedBlobs(t, "x")
	t.Setenv("PATH", gitWhoseChildKeepsThePipes(t)+string(os.PathListSeparator)+os.Getenv("PATH"))
	const wall = 500 * time.Millisecond

	started := time.Now()
	_, err := readObjects(repository, oids, snapshotBudget{wall: wall, object: 1 << 20}, func(string, []byte) error { return nil })
	elapsed := time.Since(started)

	if err == nil || !strings.Contains(err.Error(), "took longer than") {
		t.Errorf("readObjects = %v; want an error that says the read took longer than its bound", err)
	}
	if ceiling := wall + 5*time.Second; elapsed > ceiling {
		t.Errorf("readObjects took %s against a bound of %s: the read outlived its bound by waiting for a child of git", elapsed, wall)
	}
}

// gitWhoseChildKeepsThePipes is a git that starts a child for cat-file and waits for it: killing the git
// leaves the child, which holds the stream.
func gitWhoseChildKeepsThePipes(t *testing.T) string {
	t.Helper()
	real, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("find git: %v", err)
	}
	directory := t.TempDir()
	script := "#!/bin/sh\nfor argument in \"$@\"; do\n  if [ \"$argument\" = cat-file ]; then sleep 30 & wait; exit; fi\ndone\nexec '" + real + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(directory, "git"), []byte(script), 0o755); err != nil {
		t.Fatalf("write the git whose child keeps the pipes: %v", err)
	}
	return directory
}
