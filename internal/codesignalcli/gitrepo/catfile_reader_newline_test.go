package gitrepo

import (
	"bytes"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

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
	dir := gitfixture.Init(t)
	weirdPath := "weird\nname.txt"
	gitfixture.CommitFile(t, dir, "before.go", "package before\n")
	gitfixture.CommitFile(t, dir, weirdPath, "weird content\n")
	headSHA := gitfixture.CommitFile(t, dir, "after.go", "package after\n")

	reader, err := NewRevisionFileReader(dir, headSHA)
	if err != nil {
		t.Fatalf("newRevisionFileReader: unexpected error: %v", err)
	}
	defer reader.Close()

	gotBefore, err := reader.Next("before.go")
	if err != nil {
		t.Fatalf("next(before.go): unexpected error: %v", err)
	}
	if !bytes.Equal(gotBefore, []byte("package before\n")) {
		t.Errorf("next(before.go) = %q, want before.go's content", gotBefore)
	}

	gotWeird, err := reader.Next(weirdPath)
	if err != nil {
		t.Fatalf("next(%q): unexpected error: %v", weirdPath, err)
	}
	if !bytes.Equal(gotWeird, []byte("weird content\n")) {
		t.Errorf("next(%q) = %q, want its own content, not split or misattributed", weirdPath, gotWeird)
	}

	gotAfter, err := reader.Next("after.go")
	if err != nil {
		t.Fatalf("next(after.go): unexpected error: %v", err)
	}
	if !bytes.Equal(gotAfter, []byte("package after\n")) {
		t.Errorf("next(after.go) = %q, want after.go's content (proving the newline-path request/response pair didn't desync the stream)", gotAfter)
	}
}
