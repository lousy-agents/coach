package gitrepo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// aRepositoryWhoseSystemConfigurationHidesAnUntrackedFile commits one file and leaves secret.go untracked, with a
// system-wide configuration that names secret.go in its excludesFile.
func aRepositoryWhoseSystemConfigurationHidesAnUntrackedFile(t *testing.T) string {
	t.Helper()
	dir := gitfixture.Init(t)
	gitfixture.CommitFile(t, dir, "a.go", "package a\n")
	if err := os.WriteFile(filepath.Join(dir, "secret.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	excludes := filepath.Join(t.TempDir(), "excludes")
	system := filepath.Join(t.TempDir(), "system-gitconfig")
	if err := os.WriteFile(excludes, []byte("secret.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(system, []byte("[core]\n\texcludesFile = "+excludes+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_SYSTEM", system)
	return dir
}

func listsPath(entries []WorktreeEntry, path string) bool {
	for _, entry := range entries {
		if entry.Path == path {
			return true
		}
	}
	return false
}

// The canary is live: the status of a committed scan reads the system-wide configuration, and does not list the file.
func TestTheStatusOfACommittedScanHonorsTheSystemConfigurationThatHidesAnUntrackedFile(t *testing.T) {
	dir := aRepositoryWhoseSystemConfigurationHidesAnUntrackedFile(t)

	entries, err := CommittedWorktreeStatus(dir)

	if err != nil || listsPath(entries, "secret.go") {
		t.Errorf("CommittedWorktreeStatus = %+v, %v; want the status to leave out secret.go, or the test below proves nothing", entries, err)
	}
}

func TestTheStatusOfAWorktreeScanDoesNotLetTheSystemConfigurationHideAnUntrackedFile(t *testing.T) {
	dir := aRepositoryWhoseSystemConfigurationHidesAnUntrackedFile(t)

	read, err := ReadWorktreeStatus(dir)

	if err != nil || !listsPath(read.Entries, "secret.go") {
		t.Errorf("ReadWorktreeStatus = %+v, %v; want secret.go listed as untracked", read.Entries, err)
	}
}

// A status read as a git that cannot be told not to fetch has not looked into the submodules, and says which git it
// was, so that a report does not claim a tree clean when a submodule has edits.
func TestTheStatusOfAGitThatCannotBeToldNotToFetchSaysItSkippedTheSubmodules(t *testing.T) {
	dir := gitfixture.Init(t)
	gitfixture.CommitFile(t, dir, "a.go", "package a\n")

	read, err := readWorktreeStatusBy(worktreeChildOf(commandTold(nil)), dir, false, "2.43.0")

	if err != nil || read.SubmodulesSkippedBy != "2.43.0" {
		t.Errorf("readWorktreeStatusBy = %+v, %v; want SubmodulesSkippedBy 2.43.0", read, err)
	}
}

func TestTheStatusOfAGitThatCanBeToldNotToFetchSaysNothingWasSkipped(t *testing.T) {
	dir := gitfixture.Init(t)
	gitfixture.CommitFile(t, dir, "a.go", "package a\n")

	read, err := ReadWorktreeStatus(dir)

	if err != nil || read.SubmodulesSkippedBy != "" {
		t.Errorf("ReadWorktreeStatus = %+v, %v; want SubmodulesSkippedBy empty", read, err)
	}
}
