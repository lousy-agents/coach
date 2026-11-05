package gitrepo_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// hookedRepo is a repository with one dirty file whose git configuration makes
// every status run an fsmonitor hook that leaves a marker file.
func hookedRepo(t *testing.T) (dir, marker string) {
	t.Helper()
	dir = gitfixture.Init(t)
	gitfixture.CommitFile(t, dir, "a.go", "package a\n")
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a // edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	scripts := t.TempDir()
	marker = filepath.Join(scripts, "fsmonitor-ran")
	hook := filepath.Join(scripts, "fsmonitor.sh")
	if err := os.WriteFile(hook, []byte(fmt.Sprintf("#!/bin/sh\ntouch %q\nexit 1\n", marker)), 0o755); err != nil {
		t.Fatal(err)
	}
	configure := exec.Command("git", "config", "core.fsmonitor", hook)
	configure.Dir = dir
	if output, err := configure.CombinedOutput(); err != nil {
		t.Fatalf("git config: %v: %s", err, output)
	}
	return dir, marker
}

func hookRan(marker string) bool {
	_, err := os.Stat(marker)
	return err == nil
}

// A committed scan reads the status as it always has, with the repository's own
// configuration, so the hook the repository names runs. The read behind
// --worktree hardens against it and the hook does not run. Both return the dirty
// file, so the difference between them is the configuration they honour.
func TestCommittedWorktreeStatusHonorsTheRepositoryConfigurationThatWorktreeStatusIgnores(t *testing.T) {
	tests := []struct {
		name     string
		read     func(string) ([]gitrepo.WorktreeEntry, error)
		wantHook bool
	}{
		{"a committed scan runs git as it always has", gitrepo.CommittedWorktreeStatus, true},
		{"a --worktree scan hardens the read", worktreeEntries, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertStatusHookBehavior(t, tt.read, tt.wantHook)
		})
	}
}

func worktreeEntries(dir string) ([]gitrepo.WorktreeEntry, error) {
	read, err := gitrepo.ReadWorktreeStatus(dir)
	return read.Entries, err
}

func assertStatusHookBehavior(t *testing.T, read func(string) ([]gitrepo.WorktreeEntry, error), wantHook bool) {
	t.Helper()
	dir, marker := hookedRepo(t)

	entries, err := read(dir)

	if err != nil || len(entries) != 1 || entries[0].Path != "a.go" {
		t.Fatalf("status = %+v, %v; want the one dirty file a.go", entries, err)
	}
	if got := hookRan(marker); got != wantHook {
		t.Errorf("fsmonitor hook ran = %v, want %v", got, wantHook)
	}
}
