package gitrepo

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestRunBytesEndsAGitThatNeverAnswersAndNamesTheQuestion(t *testing.T) {
	runBytes := childRunnerHeldTo(budget{timeout: 300 * time.Millisecond, stdout: 1 << 20, stderr: 1 << 16}, func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, "sleep", "30")
	})

	begin := time.Now()
	_, err := runBytes(".", "rev-parse", "HEAD")
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
	runBytes := childRunnerHeldTo(budget{timeout: 10 * time.Second, stdout: 1 << 10, stderr: 1 << 16}, func(ctx context.Context) *exec.Cmd {
		// head itself, not a shell that starts it: when the read is stopped, the process that is killed must be
		// the one that holds the pipe, or the pipe stays open.
		return exec.CommandContext(ctx, "head", "-c", "100000", "/dev/zero")
	})

	_, err := runBytes(".", "show", "HEAD:big")

	var bound *BoundError
	if !errors.As(err, &bound) || bound.Kind != BoundStdout {
		t.Fatalf("error = %v, want a stdout-budget BoundError", err)
	}
}

func TestRunBytesStillReturnsWhatAnAnsweringGitPrints(t *testing.T) {
	runBytes := childRunnerHeldTo(budget{timeout: 10 * time.Second, stdout: 1 << 10, stderr: 1 << 16}, func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "printf answer")
	})

	output, err := runBytes(".", "rev-parse", "HEAD")

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
		{"the index entries of one path", []string{"ls-files", "--stage", "-z", "--full-name", "--", "x"}, lookup(30*second, 1<<20)},
		{"the version of git", []string{"version"}, lookup(30*second, 1<<20)},
		{"one object by name", []string{"cat-file", "-e", "x"}, lookup(30*second, 1<<20)},
		{"the merge base", []string{"merge-base", "a", "b"}, lookup(30*second, 1<<20)},
		{"a listing of the tree", []string{"ls-tree", "-r", "-z", "--name-only", "x"}, lookup(30*second, 64<<20)},
		{"the changed files", []string{"diff", "--name-status", "-z"}, lookup(120*second, 64<<20)},
		{"the changed lines of one file", []string{"diff", "--unified=0", "--no-ext-diff", "--no-textconv", "a", "HEAD", "--", "x"}, lookup(30*second, 32<<20)},
		{"one file's content", []string{"show", "x:y"}, lookup(30*second, 32<<20)},
		{"the missing objects of a whole tree", []string{"rev-list", "--objects", "--missing=print", "--no-walk", "--quiet", "x"}, lookup(120*second, 64<<20)},
		{"a read nobody named", []string{"something-new"}, lookup(30*second, 16<<20)},
	}
	for _, c := range cases {
		t.Run(c.name, c.run)
	}
}
