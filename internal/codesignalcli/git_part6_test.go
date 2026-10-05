package codesignalcli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDiscoverTrackedFilesIncludesUntouchedFirstCommitFile is the key
// differentiator a Repository Baseline scan exists for: a file committed at
// the repository's very first commit and never modified since would be
// invisible to any git-diff-based comparison against that same commit
// (there is no delta), but DiscoverTrackedFiles lists every tracked file at
// a revision regardless of history, so it must still appear.
func TestDiscoverTrackedFilesIncludesUntouchedFirstCommitFile(t *testing.T) {
	dir := newTempGitRepoT(t)
	commitFileT(t, dir, "untouched.go", "package untouched\n")
	headSHA := commitFileT(t, dir, "other.go", "package other\n")

	files, coverage, err := DiscoverTrackedFiles(dir, headSHA)
	if err != nil {
		t.Fatalf("DiscoverTrackedFiles: unexpected error: %v", err)
	}

	found := false
	for _, f := range files {
		if f.Path == "untouched.go" {
			found = true
		}
	}
	if !found {
		t.Errorf("DiscoverTrackedFiles(headSHA) = %#v, want untouched.go included even though it was last modified in an earlier commit", files)
	}
	if coverage.TrackedFilesDiscovered != 2 {
		t.Errorf("coverage.TrackedFilesDiscovered = %d, want 2 (untouched.go and other.go both exist at headSHA)", coverage.TrackedFilesDiscovered)
	}
}

func commitFileT(t *testing.T, dir, name, contents string) string {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}

	addCmd := exec.Command("git", "add", name)
	addCmd.Dir = dir
	if output, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git add %s: %v: %s", name, err, output)
	}

	commitCmd := exec.Command("git", "commit", "-m", "commit "+name)
	commitCmd.Dir = dir
	commitCmd.Env = commitTestEnv
	if output, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit %s: %v: %s", name, err, output)
	}

	revCmd := exec.Command("git", "rev-parse", "HEAD")
	revCmd.Dir = dir
	output, err := revCmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}

	return strings.TrimSpace(string(output))
}

func renameFileT(t *testing.T, dir, from, to string) string {
	t.Helper()

	mvCmd := exec.Command("git", "mv", from, to)
	mvCmd.Dir = dir
	if output, err := mvCmd.CombinedOutput(); err != nil {
		t.Fatalf("git mv %s %s: %v: %s", from, to, err, output)
	}

	commitCmd := exec.Command("git", "commit", "-m", "rename "+from+" to "+to)
	commitCmd.Dir = dir
	commitCmd.Env = commitTestEnv
	if output, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit rename: %v: %s", err, output)
	}

	revCmd := exec.Command("git", "rev-parse", "HEAD")
	revCmd.Dir = dir
	output, err := revCmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}

	return strings.TrimSpace(string(output))
}
