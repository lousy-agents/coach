package gitrepo

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// A reader that is cut off while it discards an object past the limit says which file, and does not call it a
// file that is too large: the answer to its request never came whole.
func TestRevisionFileReaderNamesTheFileWhoseOversizeObjectWasCutShort(t *testing.T) {
	withStreamChild(t, func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "head -c 1 >/dev/null; printf '1111 blob 99\\000abc'")
	})
	withFileReadBudget(t, fileStreamBudget{wall: time.Minute, object: 10})
	reader, err := NewRevisionFileReader("", "HEAD")
	if err != nil {
		t.Fatalf("NewRevisionFileReader: %v", err)
	}
	defer reader.Close()

	_, readErr := reader.Next("assets/blob.bin")

	if readErr == nil || !strings.Contains(readErr.Error(), `discarding cat-file content for "assets/blob.bin"`) {
		t.Errorf("Next error = %v, want the cut-short discard named by its file", readErr)
	}
}

// Close ends the git it started: closing its stdin is what makes it exit, and Close waits, so no process is left.
func TestRevisionFileReaderCloseEndsTheGitAndWaitsForIt(t *testing.T) {
	repository, _ := committedBlobs(t, "content")
	reader, err := NewRevisionFileReader(repository, "HEAD")
	if err != nil {
		t.Fatalf("NewRevisionFileReader: %v", err)
	}
	closed := make(chan struct{})

	go func() {
		reader.Close()
		close(closed)
	}()

	select {
	case <-closed:
	case <-time.After(30 * time.Second):
		t.Fatal("Close did not return: closing stdin did not end git")
	}
	if reader.cmd.ProcessState == nil || !reader.cmd.ProcessState.Success() {
		t.Errorf("process state after Close = %v, want git exited cleanly and waited for", reader.cmd.ProcessState)
	}
}

// The answer to a request is a header, the content and a NUL, each ended by a NUL. The child here prints a
// piece of one after it has read the first byte of the request, and exits 0, so that only the reader's own
// reading of what it got can say that it is not whole.
func TestRevisionFileReaderNamesWhichPartOfAnAnswerWasWrong(t *testing.T) {
	cases := []struct{ name, printed, want string }{
		{"no answer at all", "", `reading cat-file response header for "p.go"`},
		{"a size that is not a number", `1111 blob many\000`, `malformed cat-file size for "p.go"`},
		{"a size below zero", `1111 blob -1\000`, `malformed cat-file size for "p.go"`},
		{"a header of one field", `junk\000`, `malformed cat-file header for "p.go"`},
		{"a header of two fields that is not the missing marker", `1111 ambiguous\000`, `malformed cat-file header for "p.go"`},
		{"a header of four fields", `1111 blob 2 extra\000`, `malformed cat-file header for "p.go"`},
		{"an object that is not there", `1111 missing\000`, `"p.go" is missing at HEAD`},
		{"content cut short", `1111 blob 5\000ab`, `reading cat-file content for "p.go"`},
		{"no NUL after the content", `1111 blob 2\000hi`, `reading cat-file trailing NUL for "p.go"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertReaderFails(t, c.printed, c.want)
		})
	}
}

func assertReaderFails(t *testing.T, printed, want string) {
	t.Helper()
	withStreamChild(t, func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "head -c 1 >/dev/null; printf '"+printed+"'")
	})
	withFileReadBudget(t, fileStreamBudget{wall: time.Minute, object: 10})
	reader, err := NewRevisionFileReader("", "HEAD")
	if err != nil {
		t.Fatalf("NewRevisionFileReader: %v", err)
	}
	defer reader.Close()

	content, readErr := reader.Next("p.go")

	if readErr == nil || !strings.Contains(readErr.Error(), want) || content != nil {
		t.Errorf("Next = %q, %v; want no content and an error that says %q", content, readErr, want)
	}
}

// An empty file is a file: its object is a header with a size of zero and nothing after the NUL that ends the
// content. A size of zero is not malformed, and the scan reports the file as empty and not as unreadable.
func TestRevisionFileReaderReadsAnEmptyObjectAsEmptyContent(t *testing.T) {
	withStreamChild(t, func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "head -c 1 >/dev/null; printf '1111 blob 0\\000\\000'")
	})
	withFileReadBudget(t, fileStreamBudget{wall: time.Minute, object: 10})
	reader, err := NewRevisionFileReader("", "HEAD")
	if err != nil {
		t.Fatalf("NewRevisionFileReader: %v", err)
	}
	defer reader.Close()

	content, readErr := reader.Next("p.go")

	if readErr != nil || content == nil || len(content) != 0 {
		t.Errorf("Next = %q, %v; want empty content and no error", content, readErr)
	}
}
