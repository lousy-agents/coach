package gitrepo

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// A read that overruns its output budget has an answer the moment it overruns. git is blocked
// writing into a pipe nobody reads any more, so the read must end git then, not wait for the
// wall-time bound to do it.
func TestBoundedReadRefusesAnOverrunningOutputPromptly(t *testing.T) {
	dir := gitfixture.Init(t)
	gitfixture.CommitFile(t, dir, "big.txt", strings.Repeat("x", 3<<20))
	const deadline = 8 * time.Second

	start := time.Now()
	_, err := RunBytesBounded(dir, 1<<20, 1<<16, deadline, "show", "HEAD:big.txt")
	elapsed := time.Since(start)

	var bound *BoundError
	if !errors.As(err, &bound) || bound.Kind != BoundStdout {
		t.Fatalf("error = %v, want the stdout budget error", err)
	}
	if elapsed > deadline/2 {
		t.Errorf("the budget error took %s: the read waited for the %s wall-time bound instead of ending git when the output overran", elapsed, deadline)
	}
}

// The same holds for stderr: a child that writes more than its stderr budget and then blocks on the
// full pipe has already answered the read, so the read must end it then.
func TestBoundedReadRefusesAnOverrunningStderrPromptly(t *testing.T) {
	const deadline = 8 * time.Second
	writesTooMuchToStderr := func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "head -c 3145728 /dev/zero >&2")
	}

	start := time.Now()
	_, err := RunBytesBoundedWith(writesTooMuchToStderr, ".", 1<<20, 1<<16, deadline)
	elapsed := time.Since(start)

	var bound *BoundError
	if !errors.As(err, &bound) || bound.Kind != BoundStderr {
		t.Fatalf("error = %v, want the stderr budget error", err)
	}
	if elapsed > deadline/2 {
		t.Errorf("the budget error took %s: the read waited for the %s wall-time bound instead of ending the child when its stderr overran", elapsed, deadline)
	}
}
