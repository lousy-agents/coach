package gitrepo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The cat-file child here is all of git for the stream: it prints what it is given and exits 0, so that nothing
// but the reader's own reading of the answer can report that the answer was wrong. (A child that dies fails in
// Wait, which would say so for any answer.)
func childPrinting(t *testing.T, output string) func(ctx context.Context) *exec.Cmd {
	t.Helper()
	file := filepath.Join(t.TempDir(), "answer")
	if err := os.WriteFile(file, []byte(output), 0o644); err != nil {
		t.Fatal(err)
	}
	return func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "cat", file) }
}

func assertReadObjectsFails(t *testing.T, child func(ctx context.Context) *exec.Cmd, want string) {
	t.Helper()
	withStreamChild(t, child)
	visited := 0

	outcome, err := readObjects("", []string{"1111"}, snapshotBudget{wall: time.Minute, object: 10}, func(string, []byte) error {
		visited++
		return nil
	})

	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("readObjects = %+v, %v; want an error that says %q", outcome, err, want)
	}
	if visited != 0 {
		t.Errorf("an object from a broken answer was handed to the visit %d times", visited)
	}
}

func TestReadObjectsNamesWhichPartOfAnAnswerWasWrong(t *testing.T) {
	cases := []struct{ name, output, want string }{
		{"no answer at all", "", "reading the cat-file response for 1111"},
		{"a header with the wrong number of fields", "junk\n", "malformed cat-file header for 1111"},
		{"a size that is not a number", "1111 blob many\n", "malformed cat-file size for 1111"},
		{"a negative size", "1111 blob -1\n", "malformed cat-file size for 1111"},
		{"content cut short", "1111 blob 5\nab", "reading the cat-file content for 1111"},
		{"no newline after the content", "1111 blob 2\nhi", "reading the end of the cat-file content for 1111"},
		{"an object past the limit cut short", "1111 blob 99\nabc", "reading past the cat-file content for 1111"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertReadObjectsFails(t, childPrinting(t, c.output), c.want)
		})
	}
}

func TestReadObjectsReportsAGitThatExitsWithAFailureAfterAWholeAnswer(t *testing.T) {
	withStreamChild(t, func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "printf '1111 blob 2\\nhi\\n'; echo 'fatal: bad things' >&2; exit 3")
	})

	_, err := readObjects("", []string{"1111"}, snapshotBudget{wall: time.Minute, object: 10}, func(string, []byte) error { return nil })

	if err == nil || !strings.Contains(err.Error(), "exit status 3") || !strings.Contains(err.Error(), "fatal: bad things") {
		t.Errorf("readObjects error = %v; want git's exit status and what it said", err)
	}
}

func TestReadObjectsFailsWhenTheChildCannotBeConnectedOrStarted(t *testing.T) {
	cases := []struct {
		name  string
		child func(ctx context.Context) *exec.Cmd
	}{
		{"its stdin is already taken", func(ctx context.Context) *exec.Cmd {
			cmd := exec.CommandContext(ctx, "cat")
			cmd.Stdin = strings.NewReader("")
			return cmd
		}},
		{"its stdout is already taken", func(ctx context.Context) *exec.Cmd {
			cmd := exec.CommandContext(ctx, "cat")
			cmd.Stdout = &strings.Builder{}
			return cmd
		}},
		{"git is not there to start", func(ctx context.Context) *exec.Cmd {
			return exec.CommandContext(ctx, filepath.Join(t.TempDir(), "no-git"))
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertReadObjectsFails(t, c.child, "")
		})
	}
}
