package codesignalcli

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// A scan that reads each changed file from git on its own must end when git does not answer, and not wait
// out the limit once per file; a file that git printed too much of is only that file's problem.

func boundedRead(t *testing.T, child func(ctx context.Context) *exec.Cmd, maxStdout int64, timeout time.Duration) error {
	t.Helper()
	build := func(ctx context.Context, _ string, _ ...string) *exec.Cmd { return child(ctx) }
	_, err := gitrepo.RunBytesBoundedWith(build, ".", maxStdout, 1<<10, timeout, "show", "HEAD:a.go")
	if err == nil {
		t.Fatal("the bounded read did not fail")
	}
	return err
}

func TestReadFailureEndsTheScanWhenGitDoesNotAnswer(t *testing.T) {
	err := boundedRead(t, func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "sleep", "30") }, 1<<10, 200*time.Millisecond)

	diagnostics, stalled := readFailure("a.go", "head", err)

	var operational *gitrepo.OperationalError
	if !errors.As(stalled, &operational) || diagnostics != nil {
		t.Errorf("readFailure = %v, %v; want the operational error and no diagnostics", diagnostics, stalled)
	}
}

func TestReadFailureKeepsTheScanGoingForAFileGitPrintedTooMuchOf(t *testing.T) {
	err := boundedRead(t, func(ctx context.Context) *exec.Cmd {
		return exec.CommandContext(ctx, "head", "-c", "5000", "/dev/zero")
	}, 1<<10, 10*time.Second)

	diagnostics, stalled := readFailure("a.go", "head", err)

	if stalled != nil || len(diagnostics) != 1 || diagnostics[0].Kind != "head_read_failed" || diagnostics[0].Path != "a.go" {
		t.Errorf("readFailure = %v, %v; want one head_read_failed diagnostic for a.go and no error", diagnostics, stalled)
	}
}

func TestReadFailureKeepsTheScanGoingForAFileGitAnsweredNoFor(t *testing.T) {
	diagnostics, stalled := readFailure("a.go", "base", errors.New("exit status 128: fatal: path 'a.go' does not exist"))

	if stalled != nil || len(diagnostics) != 1 || diagnostics[0].Kind != "base_read_failed" {
		t.Errorf("readFailure = %v, %v; want one base_read_failed diagnostic and no error", diagnostics, stalled)
	}
}
