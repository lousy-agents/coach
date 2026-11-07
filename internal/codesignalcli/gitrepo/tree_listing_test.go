package gitrepo

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// withTreeListing replaces the git that lists a tree with a child that prints output and exits 0, and fails the
// test on any other command, so the listing under test is the only thing that runs.
func withTreeListing(t *testing.T, output string) {
	t.Helper()
	child := childPrinting(t, output)
	original := commandContext
	t.Cleanup(func() { commandContext = original })
	commandContext = func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		if args[0] != "ls-tree" {
			t.Errorf("unexpected git command %v: the fake git lists a tree", args)
		}
		return child(ctx)
	}
}

func withListingScript(t *testing.T, script string) {
	t.Helper()
	original := commandContext
	t.Cleanup(func() { commandContext = original })
	commandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", script)
	}
}

// listedRecords streams the fake git's listing and returns the records it handed over, whole, in order.
func listedRecords() ([]string, error) {
	var records []string
	err := streamListing("", time.Minute, []string{"ls-tree"}, func(record []byte) error {
		records = append(records, string(record))
		return nil
	})
	return records, err
}

func TestAListingIsHandedOverARecordAtATimeInOrder(t *testing.T) {
	withTreeListing(t, "one\x00two words\x00three\nwith a newline\x00")

	records, err := listedRecords()

	if err != nil || strings.Join(records, "|") != "one|two words|three\nwith a newline" {
		t.Errorf("streamListing = %v after %q; want the three records, whole", err, records)
	}
}

func TestAVisitThatFailsEndsGitAndItsErrorIsReturned(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	withListingScript(t, "echo $$ > '"+pidFile+"'; while :; do printf 'aaaaaaaa\\000'; done")
	failure := errors.New("the visit failed")

	err := streamListing("", time.Minute, []string{"ls-tree"}, func([]byte) error { return failure })

	if !errors.Is(err, failure) {
		t.Errorf("streamListing = %v; want the error of the visit, not a timeout", err)
	}
	pid, readErr := os.ReadFile(pidFile)
	if readErr != nil {
		t.Fatalf("the child did not record its process: %v", readErr)
	}
	if exec.Command("kill", "-0", strings.TrimSpace(string(pid))).Run() == nil {
		t.Errorf("git (process %s) is still running after a visit failed: it blocks writing to a pipe nothing reads", strings.TrimSpace(string(pid)))
	}
}

func TestAGitThatCannotBeStartedIsAnError(t *testing.T) {
	original := commandContext
	t.Cleanup(func() { commandContext = original })
	commandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "/no/such/git")
	}

	if err := streamListing("", time.Minute, []string{"ls-tree"}, func([]byte) error { return nil }); err == nil || !strings.Contains(err.Error(), "git ls-tree") {
		t.Errorf("streamListing = %v; want the failure to start git, naming ls-tree", err)
	}
}
