package gitrepo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// An object past the limit on one object is not held in memory. When a path that names it is a file the scan
// reads, it is written to disk a chunk at a time and in full: go list parses a .go file, and an empty file in
// its place fails the parse with an error of its own ("expected 'package', found 'EOF'") where v0.11.0 went on
// and the analyzer named the file too large. The file counts against the total like any other.

const oversizeObject = 50

func extractedAt(t *testing.T, repository string) string {
	t.Helper()
	dir, err := ExtractRevision(repository, "HEAD")
	if err != nil {
		t.Fatalf("ExtractRevision = %v, want nil", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func contentOf(t *testing.T, dir, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(content)
}

func TestExtractRevisionWritesInFullAFileTheScanReadsThatPassesTheLimitOnOneObject(t *testing.T) {
	goSource := "package big\n\n// " + strings.Repeat("g", 100) + "\n"
	jsonData := `{"data":"` + strings.Repeat("j", 100) + `"}`
	repository, _ := snapshotOfFiles(t, namedFile{"dom/big.go", goSource}, namedFile{"cfg/big.json", jsonData}, namedFile{"small.txt", "small"})
	withSnapshotLimits(t, oversizeObject, snapshotTotalBytes)

	dir := extractedAt(t, repository)

	for name, want := range map[string]string{"dom/big.go": goSource, "cfg/big.json": jsonData, "small.txt": "small"} {
		if got := contentOf(t, dir, name); got != want {
			t.Errorf("%s is %d bytes, want its %d: a file the scan reads is written in full, whatever its size", name, len(got), len(want))
		}
	}
}

// The stream stays in step: after an object that is read to disk in chunks, and around one that is read past,
// the next response is the next object.
func TestExtractRevisionKeepsTheStreamInStepAroundOversizeObjects(t *testing.T) {
	big := strings.Repeat("b", 100)
	repository, _ := snapshotOfFiles(t, namedFile{"a.go", "package a // " + big}, namedFile{"b.png", big}, namedFile{"c.go", "package c"}, namedFile{"d.go", "package d // " + big})
	withSnapshotLimits(t, oversizeObject, snapshotTotalBytes)

	dir := extractedAt(t, repository)

	want := map[string]string{"a.go": "package a // " + big, "c.go": "package c", "d.go": "package d // " + big, "b.png": ""}
	for name, content := range want {
		if got := contentOf(t, dir, name); got != content {
			t.Errorf("%s = %d bytes, want %d", name, len(got), len(content))
		}
	}
}

func TestExtractRevisionWritesEveryPathOfAnObjectPastTheLimitThatOneOfThemNeeds(t *testing.T) {
	source := "package same // " + strings.Repeat("s", 100)
	repository, _ := snapshotOfFiles(t, namedFile{"a.go", source}, namedFile{"copy/b.go", source}, namedFile{"asset.png", source})
	withSnapshotLimits(t, oversizeObject, snapshotTotalBytes)

	dir := extractedAt(t, repository)

	for _, name := range []string{"a.go", "copy/b.go", "asset.png"} {
		if got := contentOf(t, dir, name); got != source {
			t.Errorf("%s is %d bytes, want its %d: one object, named by a path the scan reads, is written at every path that names it", name, len(got), len(source))
		}
	}
}

func TestExtractRevisionStillDropsTheBytesOfAnObjectPastTheLimitThatNothingReads(t *testing.T) {
	repository, _ := snapshotOfFiles(t, namedFile{"assets/blob.bin", strings.Repeat("x", 100)}, namedFile{"main.go", "package main"})
	withSnapshotLimits(t, oversizeObject, snapshotTotalBytes)

	dir := extractedAt(t, repository)

	if got := sizeOf(t, dir, "assets/blob.bin"); got != 0 {
		t.Errorf("assets/blob.bin is %d bytes, want a zero-length file: no analyzer reads it", got)
	}
}

// A link has no stream form: its content is its target. A link whose target is past the limit is left out, as a
// placeholder leaves it out, and is not written as a file that holds the target.
func TestExtractRevisionDoesNotWriteALinkNamedLikeASourceAsAFile(t *testing.T) {
	repository, _ := snapshotOfFiles(t, namedFile{"main.go", "package main"})
	gitfixture.CommitLink(t, repository, "alias.go", strings.Repeat("t", 100))
	withSnapshotLimits(t, oversizeObject, snapshotTotalBytes)

	dir := extractedAt(t, repository)

	if info, err := os.Lstat(filepath.Join(dir, "alias.go")); err == nil {
		t.Errorf("alias.go exists (%s), want nothing: a link whose target is past the limit has no form to be written in", info.Mode())
	}
}
