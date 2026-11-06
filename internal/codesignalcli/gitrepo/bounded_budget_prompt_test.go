package gitrepo

import (
	"errors"
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
