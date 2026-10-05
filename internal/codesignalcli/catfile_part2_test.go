package codesignalcli

import (
	"bytes"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// TestRevisionFileReaderReadsFilesInRequestOrder is the acceptance test for
// the new git-cat-file-batch-backed content reader that replaces
// AnalyzeBaseline's old one-`git show`-subprocess-per-file loop. It exists
// specifically to catch the new failure mode a single streaming batch
// process can introduce that separate `git show` subprocesses never could:
// a missing/failed object's response getting misread in a way that leaves
// the next file's content (or its "missing" marker) misaligned with the
// requests already sent. It interleaves a present file, a missing file,
// and another present file and asserts each next() call returns exactly
// the bytes for the path it was asked for -- proving state isn't corrupted
// across a failure in the middle of a batch.
func TestRevisionFileReaderReadsFilesInRequestOrder(t *testing.T) {
	dir := gitfixture.Init(t)
	gitfixture.CommitFile(t, dir, "a.go", "package a\n\nfunc A() {}\n")
	headSHA := gitfixture.CommitFile(t, dir, "b.go", "package b\n\nfunc B() {}\n")

	reader, err := newRevisionFileReader(dir, headSHA)
	if err != nil {
		t.Fatalf("newRevisionFileReader: unexpected error: %v", err)
	}
	defer reader.close()

	gotA, err := reader.next("a.go")
	if err != nil {
		t.Fatalf("next(a.go): unexpected error: %v", err)
	}
	if !bytes.Equal(gotA, []byte("package a\n\nfunc A() {}\n")) {
		t.Errorf("next(a.go) = %q, want a.go's content", gotA)
	}

	if _, err := reader.next("missing.go"); err == nil {
		t.Errorf("next(missing.go): want error, got nil")
	}

	gotB, err := reader.next("b.go")
	if err != nil {
		t.Fatalf("next(b.go): unexpected error: %v", err)
	}
	if !bytes.Equal(gotB, []byte("package b\n\nfunc B() {}\n")) {
		t.Errorf("next(b.go) = %q, want b.go's content", gotB)
	}
}
