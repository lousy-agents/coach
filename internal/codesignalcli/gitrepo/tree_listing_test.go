package gitrepo

import (
	"context"
	"errors"
	"io"
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

func visitingEvery(records *[]string) func([]byte) error {
	return func(record []byte) error {
		*records = append(*records, string(record))
		return nil
	}
}

func TestAListingIsHandedOverARecordAtATimeInOrder(t *testing.T) {
	withTreeListing(t, "one\x00two words\x00three\nwith a newline\x00")
	var records []string

	err := streamListing("", time.Minute, []string{"ls-tree"}, visitingEvery(&records))

	if err != nil || strings.Join(records, "|") != "one|two words|three\nwith a newline" {
		t.Errorf("streamListing = %v after %q; want the three records, whole", err, records)
	}
}

// The listing of a large repository is past any limit on one read, and it is read all the same: what is held is
// a record. This one is 70 MB, past the 64 MiB a git read is held to.
func TestAListingPastTheBudgetOfOneReadIsReadWhole(t *testing.T) {
	withListingScript(t, "yes aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa | head -n 700000 | tr '\\n' '\\000'")
	var bytes int

	err := streamListing("", time.Minute, []string{"ls-tree"}, func(record []byte) error {
		bytes += len(record) + 1
		return nil
	})

	if err != nil || bytes != 700000*(len("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")+1) {
		t.Errorf("streamListing = %v after %d bytes; want all 700000 records read, without an error about a size", err, bytes)
	}
	if bytes <= 64<<20 {
		t.Errorf("the listing is %d bytes, which is not past the 64 MiB a git read is held to", bytes)
	}
}

func TestAListingThatEndsInsideARecordIsAnError(t *testing.T) {
	withTreeListing(t, "whole\x00cut sho")
	var records []string

	err := streamListing("", time.Minute, []string{"ls-tree"}, visitingEvery(&records))

	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("streamListing = %v after %q; want the stream cut inside a record refused", err, records)
	}
}

func TestARecordPastTheLimitIsRefusedAndGitIsEnded(t *testing.T) {
	withListingScript(t, "yes aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa | tr -d '\\n'")
	started := time.Now()

	err := streamListing("", time.Minute, []string{"ls-tree"}, func([]byte) error { return nil })

	if err == nil || !strings.Contains(err.Error(), "longer than 65536 bytes") {
		t.Errorf("streamListing = %v; want the record that never ends refused", err)
	}
	if time.Since(started) > 15*time.Second {
		t.Errorf("streamListing took %s to refuse a record: git was left running", time.Since(started))
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

func TestAGitThatDoesNotAnswerInTimeIsABoundError(t *testing.T) {
	withListingScript(t, "sleep 120")

	err := streamListing("", 300*time.Millisecond, []string{"ls-tree"}, func([]byte) error { return nil })

	var bound *BoundError
	if !errors.As(err, &bound) || bound.Kind != BoundTimeout || !strings.Contains(err.Error(), "git ls-tree") {
		t.Errorf("streamListing = %v; want a timeout BoundError that names git ls-tree", err)
	}
}

func TestAGitThatFailsIsReportedWithWhatItSaidOnStderrHeldToTheBudget(t *testing.T) {
	withListingScript(t, "printf 'whole\\000'; head -c 200000 /dev/zero | tr '\\000' 'e' >&2; exit 3")

	err := streamListing("", time.Minute, []string{"ls-tree"}, func([]byte) error { return nil })

	if err == nil || !strings.Contains(err.Error(), "exit status 3") || !strings.Contains(err.Error(), "eeee") {
		t.Fatalf("streamListing = %v; want git's exit status and what it said", err)
	}
	if len(err.Error()) > gitStderrBudget+200 {
		t.Errorf("the error is %d bytes; what git said on stderr is held to %d", len(err.Error()), gitStderrBudget)
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
