package codesignalcli

import (
	"bytes"
	"testing"
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
	dir := newTempGitRepoT(t)
	commitFileT(t, dir, "a.go", "package a\n\nfunc A() {}\n")
	headSHA := commitFileT(t, dir, "b.go", "package b\n\nfunc B() {}\n")

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

	// The failed request above must not have left stray bytes in the
	// stream: this next() call must still return b.go's own content, not
	// a.go's (proving no bytes leaked backward) and not an error carried
	// over from the missing-file response (proving no bytes leaked
	// forward).
	gotB, err := reader.next("b.go")
	if err != nil {
		t.Fatalf("next(b.go): unexpected error: %v", err)
	}
	if !bytes.Equal(gotB, []byte("package b\n\nfunc B() {}\n")) {
		t.Errorf("next(b.go) = %q, want b.go's content", gotB)
	}
}

// TestRevisionFileReaderHandlesPathContainingNewline proves the batch
// reader survives a tracked Git path that itself contains a literal
// newline (Git permits this; cmd/coach/acceptance_test.go already proves
// the --base path preserves such a path exactly). Without the -Z flag,
// `git cat-file --batch`'s newline-delimited protocol would treat the
// embedded newline as ending the request early, splitting one legal path
// into two bogus object identifiers and desyncing every response after it
// for the rest of the batch -- this is the failure mode this test guards
// against.
func TestRevisionFileReaderHandlesPathContainingNewline(t *testing.T) {
	dir := newTempGitRepoT(t)
	weirdPath := "weird\nname.txt"
	commitFileT(t, dir, "before.go", "package before\n")
	commitFileT(t, dir, weirdPath, "weird content\n")
	headSHA := commitFileT(t, dir, "after.go", "package after\n")

	reader, err := newRevisionFileReader(dir, headSHA)
	if err != nil {
		t.Fatalf("newRevisionFileReader: unexpected error: %v", err)
	}
	defer reader.close()

	gotBefore, err := reader.next("before.go")
	if err != nil {
		t.Fatalf("next(before.go): unexpected error: %v", err)
	}
	if !bytes.Equal(gotBefore, []byte("package before\n")) {
		t.Errorf("next(before.go) = %q, want before.go's content", gotBefore)
	}

	gotWeird, err := reader.next(weirdPath)
	if err != nil {
		t.Fatalf("next(%q): unexpected error: %v", weirdPath, err)
	}
	if !bytes.Equal(gotWeird, []byte("weird content\n")) {
		t.Errorf("next(%q) = %q, want its own content, not split or misattributed", weirdPath, gotWeird)
	}

	gotAfter, err := reader.next("after.go")
	if err != nil {
		t.Fatalf("next(after.go): unexpected error: %v", err)
	}
	if !bytes.Equal(gotAfter, []byte("package after\n")) {
		t.Errorf("next(after.go) = %q, want after.go's content (proving the newline-path request/response pair didn't desync the stream)", gotAfter)
	}
}
