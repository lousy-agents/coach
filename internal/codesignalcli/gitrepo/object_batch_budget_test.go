package gitrepo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// committedBlobs commits one file per content and returns the repository and the blobs' object ids.
func committedBlobs(t *testing.T, contents ...string) (repository string, oids []string) {
	t.Helper()
	repository = gitfixture.Init(t)
	for i, content := range contents {
		name := "f" + string(rune('a'+i)) + ".txt"
		gitfixture.CommitFile(t, repository, name, content)
		oids = append(oids, strings.TrimSpace(gitOutput(t, repository, "rev-parse", "HEAD:"+name)))
	}
	return repository, oids
}

// An object past the limit is read past without being held, and reported; the read goes on.
func TestReadObjectsReadsPastAnObjectPastTheLimitAndGoesOn(t *testing.T) {
	repository, oids := committedBlobs(t, strings.Repeat("x", 100), "small")
	var visited []string

	outcome, err := readObjects(repository, oids, snapshotBudget{wall: time.Minute, object: 50}, func(oid string, content []byte) error {
		visited = append(visited, string(content))
		return nil
	})

	if err != nil || len(outcome.tooLarge) != 1 || outcome.tooLarge[0] != oids[0] || len(outcome.missing) != 0 {
		t.Errorf("readObjects = %+v, %v; want the 100-byte object reported as too large", outcome, err)
	}
	if len(visited) != 1 || visited[0] != "small" {
		t.Errorf("visited %q; want only %q: the stream must stay in step after an object that is read past", visited, "small")
	}
}

// The snapshot holds one object at a time, so the limit is on an object and not on the total: a repository
// of many objects, each within the limit, is read.
func TestReadObjectsAppliesTheLimitToEachObjectAndNotToTheirSum(t *testing.T) {
	repository, oids := committedBlobs(t, strings.Repeat("a", 50), strings.Repeat("b", 50), strings.Repeat("c", 50))
	var visited int

	outcome, err := readObjects(repository, oids, snapshotBudget{wall: time.Minute, object: 50}, func(string, []byte) error {
		visited++
		return nil
	})

	if err != nil || len(outcome.missing) != 0 || len(outcome.tooLarge) != 0 || visited != 3 {
		t.Errorf("readObjects = %+v, %v after %d objects; want all three read: each is exactly at the limit", outcome, err, visited)
	}
}

// A git that stops answering ends with the wall bound and not with the process. The ceiling is the bound
// plus the time a kill and a read of the closed pipe take, never a number chosen because it looks quick.
func TestReadObjectsEndsWhenGitStopsAnswering(t *testing.T) {
	repository, oids := committedBlobs(t, "x")
	t.Setenv("PATH", gitThatStallsOnCatFile(t)+string(os.PathListSeparator)+os.Getenv("PATH"))
	const wall = 500 * time.Millisecond

	started := time.Now()
	_, err := readObjects(repository, oids, snapshotBudget{wall: wall, object: 1 << 20}, func(string, []byte) error { return nil })
	elapsed := time.Since(started)

	if err == nil || !strings.Contains(err.Error(), "took longer than") {
		t.Errorf("readObjects = %v; want an error that says the read took longer than its bound", err)
	}
	if ceiling := wall + 15*time.Second; elapsed > ceiling {
		t.Errorf("readObjects took %s against a bound of %s: the read outlived its bound", elapsed, wall)
	}
}

func gitThatStallsOnCatFile(t *testing.T) string {
	t.Helper()
	real, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("find git: %v", err)
	}
	directory := t.TempDir()
	script := "#!/bin/sh\nfor argument in \"$@\"; do\n  if [ \"$argument\" = cat-file ]; then exec sleep 120; fi\ndone\nexec '" + real + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(directory, "git"), []byte(script), 0o755); err != nil {
		t.Fatalf("write the git that stalls: %v", err)
	}
	return directory
}
