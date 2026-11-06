package gitrepo

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Run and RunBytes are what the scan's own git reads go through. A read that has no time limit waits for
// as long as git does, and a named pipe where git expects .git/HEAD makes that forever. So both end a
// git that does not answer, and a git that prints more than its read may hold.

// withGit holds every bounded read of the package to replacement and answers each with the child that
// answer builds from the git arguments, in place of git.
func withGit(t *testing.T, replacement budget, answer func(ctx context.Context, args []string) *exec.Cmd) {
	t.Helper()
	originalBudget, originalCommand := budgetFor, commandContext
	t.Cleanup(func() { budgetFor, commandContext = originalBudget, originalCommand })
	budgetFor = func([]string) budget { return replacement }
	commandContext = func(ctx context.Context, _ string, args ...string) *exec.Cmd { return answer(ctx, args) }
}

func withBudget(t *testing.T, replacement budget, child func(ctx context.Context) *exec.Cmd) {
	t.Helper()
	withGit(t, replacement, func(ctx context.Context, _ []string) *exec.Cmd { return child(ctx) })
}

func TestRunBytesEndsAGitThatNeverAnswersAndNamesTheQuestion(t *testing.T) {
	withBudget(t, budget{timeout: 300 * time.Millisecond, stdout: 1 << 20, stderr: 1 << 16}, func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, "sleep", "30")
	})

	begin := time.Now()
	_, err := RunBytes(".", "rev-parse", "HEAD")
	elapsed := time.Since(begin)

	var bound *BoundError
	if !errors.As(err, &bound) || bound.Kind != BoundTimeout {
		t.Fatalf("error = %v, want a timeout BoundError", err)
	}
	if !strings.Contains(err.Error(), "git rev-parse") {
		t.Errorf("error = %q, want it to name the git question (git rev-parse)", err)
	}
	if elapsed > 5*time.Second {
		t.Errorf("RunBytes took %s to end a git that never answers", elapsed)
	}
}

func TestRunBytesRefusesWhatGitPrintsBeyondTheReadsBudget(t *testing.T) {
	withBudget(t, budget{timeout: 10 * time.Second, stdout: 1 << 10, stderr: 1 << 16}, func(ctx context.Context) *exec.Cmd {
		// head itself, not a shell that starts it: when the read is stopped, the process that is killed must be
		// the one that holds the pipe, or the pipe stays open.
		return exec.CommandContext(ctx, "head", "-c", "100000", "/dev/zero")
	})

	_, err := RunBytes(".", "show", "HEAD:big")

	var bound *BoundError
	if !errors.As(err, &bound) || bound.Kind != BoundStdout {
		t.Fatalf("error = %v, want a stdout-budget BoundError", err)
	}
}

func TestRunBytesStillReturnsWhatAnAnsweringGitPrints(t *testing.T) {
	withBudget(t, budget{timeout: 10 * time.Second, stdout: 1 << 10, stderr: 1 << 16}, func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "printf answer")
	})

	output, err := RunBytes(".", "rev-parse", "HEAD")

	if err != nil || string(output) != "answer" {
		t.Errorf("RunBytes = %q, %v; want the answer", output, err)
	}
}

// The budgets are the numbers the scan runs under; the table pins them so that a change is a decision.
type budgetCase struct {
	name string
	args []string
	want budget
}

func (c budgetCase) run(t *testing.T) {
	if got := defaultBudgetFor(c.args); got != c.want {
		t.Errorf("budget for git %v = %+v, want %+v", c.args, got, c.want)
	}
}

func lookup(timeout time.Duration, stdout int64) budget {
	return budget{timeout: timeout, stdout: stdout, stderr: 64 << 10}
}

func TestEveryGitReadOnTheScanPathHasABudget(t *testing.T) {
	const second = time.Second
	cases := []budgetCase{
		{"a metadata read", []string{"rev-parse", "HEAD"}, lookup(30*second, 1<<20)},
		{"the partial-clone question", []string{"config", "--get-regexp", "x"}, lookup(30*second, 1<<20)},
		{"the merge base", []string{"merge-base", "a", "b"}, lookup(30*second, 1<<20)},
		{"a listing of the tree", []string{"ls-tree", "-r", "-z", "--name-only", "x"}, lookup(30*second, 64<<20)},
		{"the changed files", []string{"diff", "--name-status", "-z"}, lookup(120*second, 64<<20)},
		{"the changed lines of one file", []string{"diff", "--unified=0", "--no-ext-diff", "--no-textconv", "a", "HEAD", "--", "x"}, lookup(30*second, 32<<20)},
		{"one file's content", []string{"show", "x:y"}, lookup(30*second, 32<<20)},
		{"a read nobody named", []string{"something-new"}, lookup(30*second, 16<<20)},
	}
	for _, c := range cases {
		t.Run(c.name, c.run)
	}
}

// The snapshot stream is not a RunBytes read, so its limits are constants the stream's own code takes; the
// numbers are pinned here, with the table, so that a change is a decision.
func TestTheSnapshotStreamHasAWallTimeAnObjectSizeAndATotal(t *testing.T) {
	if SnapshotWallTime != 10*time.Minute || SnapshotObjectBytes != 512<<20 || SnapshotTotalBytes != 4<<30 {
		t.Errorf("snapshot limits = %s, %d bytes per object, %d bytes in all; want 10m0s, %d, %d", SnapshotWallTime, int64(SnapshotObjectBytes), int64(SnapshotTotalBytes), int64(512<<20), int64(4<<30))
	}
	if defaultSnapshotBudget.wall != SnapshotWallTime || defaultSnapshotBudget.object != SnapshotObjectBytes {
		t.Errorf("the default snapshot budget = %+v, want the wall time and object size above", defaultSnapshotBudget)
	}
}
