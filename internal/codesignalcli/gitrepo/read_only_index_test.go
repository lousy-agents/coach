package gitrepo

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// A scan reads the working tree; it does not maintain the user's repository. `git status` refreshes stat
// information and takes the index lock to write it back unless told not to, which rewrites .git/index
// and runs a post-index-change hook the repository carries.
func TestGitReadsLeaveTheIndexAndTheHooksAlone(t *testing.T) {
	for _, read := range gitReads() {
		t.Run(read.name, func(t *testing.T) {
			assertReadLeavesTheIndexAndHooksAlone(t, read)
		})
	}
}

func assertReadLeavesTheIndexAndHooksAlone(t *testing.T, read gitRead) {
	t.Helper()
	dir, marker := repositoryWithATouchedFileAndAnIndexHook(t)
	before := readIndex(t, dir)

	if err := read.read(dir); err != nil {
		t.Fatalf("read: %v", err)
	}

	assertIndexAndHookUntouched(t, dir, marker, before)
}

// repositoryWithATouchedFileAndAnIndexHook has a file with the same bytes and a new modification time,
// exactly what a refresh rewrites, and a post-index-change hook that leaves a marker when it runs.
func repositoryWithATouchedFileAndAnIndexHook(t *testing.T) (dir, marker string) {
	t.Helper()
	dir = gitfixture.Init(t)
	gitfixture.CommitFile(t, dir, "a.go", "package a\n")
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "a.go"), later, later); err != nil {
		t.Fatalf("touch: %v", err)
	}
	marker = filepath.Join(t.TempDir(), "hook-ran")
	hook := filepath.Join(dir, ".git", "hooks", "post-index-change")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0o755); err != nil {
		t.Fatalf("write hook: %v", err)
	}
	return dir, marker
}

func assertIndexAndHookUntouched(t *testing.T, dir, marker string, before []byte) {
	t.Helper()
	if _, err := os.Stat(marker); err == nil {
		t.Errorf("git ran the repository's post-index-change hook: a read-only scan must not write the index")
	}
	if !bytes.Equal(before, readIndex(t, dir)) {
		t.Errorf(".git/index changed during a read-only scan")
	}
}

func readIndex(t *testing.T, dir string) []byte {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--git-path", "index").Output()
	if err != nil {
		t.Fatalf("rev-parse --git-path index: %v", err)
	}
	path := string(bytes.TrimSpace(out))
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	return data
}
