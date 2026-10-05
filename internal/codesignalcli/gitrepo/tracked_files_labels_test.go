package gitrepo

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// TestDiscoverTrackedFilesLabelsExtensionlessFiles proves an extensionless
// tracked file (e.g. LICENSE, Makefile) -- for which filepath.Ext returns ""
// -- is tallied under a stable, non-empty CoverageGroup.Language rather than
// an empty string that would be omitted from JSON and render as a blank in
// text output.
func TestDiscoverTrackedFilesLabelsExtensionlessFiles(t *testing.T) {
	dir := gitfixture.Init(t)
	gitfixture.CommitFile(t, dir, "a.go", "package a\n")
	headSHA := gitfixture.CommitFile(t, dir, "LICENSE", "MIT\n")

	_, coverage, err := DiscoverTrackedFiles(dir, headSHA)
	if err != nil {
		t.Fatalf("DiscoverTrackedFiles: unexpected error: %v", err)
	}

	if len(coverage.Unsupported) != 1 {
		t.Fatalf("coverage.Unsupported = %#v, want exactly one group", coverage.Unsupported)
	}
	group := coverage.Unsupported[0]
	if group.Language == "" {
		t.Errorf("group.Language is empty, want a stable non-empty label for an extensionless file")
	}
	if group.Count != 1 {
		t.Errorf("group.Count = %d, want 1", group.Count)
	}
}

// TestDiscoverTrackedFilesIncludesUntouchedFirstCommitFile is the key
// differentiator a Repository Baseline scan exists for: a file committed at
// the repository's very first commit and never modified since would be
// invisible to any git-diff-based comparison against that same commit
// (there is no delta), but DiscoverTrackedFiles lists every tracked file at
// a revision regardless of history, so it must still appear.
func TestDiscoverTrackedFilesIncludesUntouchedFirstCommitFile(t *testing.T) {
	dir := gitfixture.Init(t)
	gitfixture.CommitFile(t, dir, "untouched.go", "package untouched\n")
	headSHA := gitfixture.CommitFile(t, dir, "other.go", "package other\n")

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
