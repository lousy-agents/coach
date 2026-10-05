package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

func TestDiscoverTrackedFilesTalliesUnsupportedByExtension(t *testing.T) {
	dir := gitfixture.Init(t)
	gitfixture.CommitFile(t, dir, "a.go", "package a\n")
	gitfixture.CommitFile(t, dir, "notes.txt", "hello\n")
	gitfixture.CommitFile(t, dir, "readme.md", "# hi\n")
	headSHA := gitfixture.CommitFile(t, dir, "other.md", "# hi again\n")

	files, coverage, err := DiscoverTrackedFiles(dir, headSHA)
	if err != nil {
		t.Fatalf("DiscoverTrackedFiles: unexpected error: %v", err)
	}

	if coverage.TrackedFilesDiscovered != 4 {
		t.Errorf("coverage.TrackedFilesDiscovered = %d, want 4", coverage.TrackedFilesDiscovered)
	}

	if len(files) != 1 || files[0].Path != "a.go" {
		t.Errorf("files = %#v, want only a.go", files)
	}

	groupCounts := map[string]int{}
	for _, g := range coverage.Unsupported {
		if g.Reason != "unsupported_language" {
			t.Errorf("unsupported group reason = %q, want unsupported_language", g.Reason)
		}
		groupCounts[g.Language] = g.Count
	}
	if groupCounts[".txt"] != 1 {
		t.Errorf("groupCounts[.txt] = %d, want 1", groupCounts[".txt"])
	}
	if groupCounts[".md"] != 2 {
		t.Errorf("groupCounts[.md] = %d, want 2", groupCounts[".md"])
	}
}

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
