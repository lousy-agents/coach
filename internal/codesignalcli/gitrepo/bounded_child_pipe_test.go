package gitrepo

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"
)

// A bound on a git read is a bound on the read, not on the one process the context can kill. When git has
// started a child that keeps its stdout and stderr open (a credential helper, a filter, a pager), killing
// git does not end the stream, and a read that waits for the stream to end waits for the child.

const childHoldsThePipe = "sleep 20 & wait"

func gitWithAChildHoldingThePipe(ctx context.Context, _ string, _ ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "sh", "-c", childHoldsThePipe)
}

func TestRunBytesBoundedEndsAtItsTimeoutWhenAChildOfGitKeepsThePipeOpen(t *testing.T) {
	begin := time.Now()
	_, err := RunBytesBoundedWith(gitWithAChildHoldingThePipe, ".", 1<<10, 1<<10, 300*time.Millisecond, "show", "HEAD:x")
	elapsed := time.Since(begin)

	var bound *BoundError
	if !errors.As(err, &bound) || bound.Kind != BoundTimeout {
		t.Fatalf("error = %v, want a timeout BoundError (the read ended with the child, not with its limit)", err)
	}
	if elapsed > 5*time.Second {
		t.Errorf("the read took %s to give up a git whose child holds the pipe, with a limit of 300ms", elapsed)
	}
}

func TestRunBytesBoundedKeepsWhatAChildlessGitAnsweredWhenItEndsInTime(t *testing.T) {
	answering := func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "printf answer")
	}

	output, err := RunBytesBoundedWith(answering, ".", 1<<10, 1<<10, 10*time.Second, "rev-parse", "HEAD")

	if err != nil || string(output) != "answer" {
		t.Errorf("RunBytesBoundedWith = %q, %v; want the answer: waiting out a pipe only when the read is over must not cut an answering git short", output, err)
	}
}

func TestRunBytesBoundedRefusesAnOverrunAtOnceWhenAChildOfGitKeepsAPipeOpen(t *testing.T) {
	// A child that keeps stderr open starts first, then 5000 bytes go to stdout, over a 1 KiB budget; the
	// read is ended when the budget is crossed, and sh is killed while the child still holds stderr.
	overrunning := func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "sleep 20 >&2 & head -c 5000 /dev/zero; wait")
	}

	begin := time.Now()
	_, err := RunBytesBoundedWith(overrunning, ".", 1<<10, 1<<10, 10*time.Second, "show", "HEAD:x")
	elapsed := time.Since(begin)

	var bound *BoundError
	if !errors.As(err, &bound) || bound.Kind != BoundStdout {
		t.Fatalf("error = %v, want a stdout-budget BoundError", err)
	}
	if elapsed > 5*time.Second {
		t.Errorf("the read took %s to refuse an overrun while a child of git held stderr", elapsed)
	}
}
