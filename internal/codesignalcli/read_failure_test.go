package codesignalcli

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
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

	diagnostics, stalled := readFailure("a.go", headSide, err)

	var operational *gitrepo.OperationalError
	if !errors.As(stalled, &operational) || diagnostics != nil {
		t.Errorf("readFailure = %v, %v; want the operational error and no diagnostics", diagnostics, stalled)
	}
}

// tooLargeCase is one place a read can be of: the side it names and the kind a file git printed too much of gets there.
type tooLargeCase struct {
	name string
	side fileSide
	want string
}

func (c tooLargeCase) run(t *testing.T) {
	err := boundedRead(t, fiveThousandZeros.command, 1<<10, 10*time.Second)

	diagnostics, stalled := readFailure("a.go", c.side, err)

	if stalled != nil || len(diagnostics) != 1 || diagnostics[0].Kind != c.want || diagnostics[0].Path != "a.go" {
		t.Errorf("readFailure = %v, %v; want one %s diagnostic for a.go and no error", diagnostics, stalled, c.want)
	}
}

// childProcess is a program that stands in for git: the whole of it, so that no shell forks it.
type childProcess struct {
	program string
	args    []string
}

func (c childProcess) command(ctx context.Context) *exec.Cmd {
	return exec.CommandContext(ctx, c.program, c.args...)
}

var fiveThousandZeros = childProcess{program: "head", args: []string{"-c", "5000", "/dev/zero"}}

// dd writes to stderr without a shell: a shell would fork it, and the fork would keep the pipe open on Linux.
var fiveThousandBytesToStderr = childProcess{program: "dd", args: []string{"if=/dev/zero", "of=/dev/stderr", "bs=5000", "count=1"}}

func TestReadFailureNamesAFileGitPrintedTooMuchOfAsTooLargeWhereTheAnalyzerWould(t *testing.T) {
	cases := []tooLargeCase{
		{"the head side of a changed file", headSide, "file_too_large"},
		{"a removed file", removedBaseSide, "file_too_large"},
		{"the base side of a modified file", modifiedBaseSide, "base_analysis_failed"},
	}
	for _, c := range cases {
		t.Run(c.name, c.run)
	}
}

func TestReadFailureDoesNotCallAFileTooLargeWhenGitPrintedTooMuchToStderr(t *testing.T) {
	err := boundedRead(t, fiveThousandBytesToStderr.command, 1<<10, 10*time.Second)

	diagnostics, stalled := readFailure("a.go", headSide, err)

	if stalled != nil || len(diagnostics) != 1 || diagnostics[0].Kind != "head_read_failed" {
		t.Errorf("readFailure = %v, %v; want one head_read_failed diagnostic and no error: it was git's complaint that was too long, not the file", diagnostics, stalled)
	}
}

func TestReadFailureKeepsTheScanGoingForAFileGitAnsweredNoFor(t *testing.T) {
	diagnostics, stalled := readFailure("a.go", modifiedBaseSide, errors.New("exit status 128: fatal: path 'a.go' does not exist"))

	if stalled != nil || len(diagnostics) != 1 || diagnostics[0].Kind != "base_read_failed" {
		t.Errorf("readFailure = %v, %v; want one base_read_failed diagnostic and no error", diagnostics, stalled)
	}
}

func TestBaselineReadFailureNamesAFileTooLargeLikeTheAnalyzerDoes(t *testing.T) {
	err := fmt.Errorf("reading: %w", &gitrepo.ObjectTooLargeError{Path: "big.go", Size: 100, Limit: 50})

	diagnostics, stalled := baselineReadFailure("big.go", err)

	if stalled != nil || len(diagnostics) != 1 || diagnostics[0].Kind != "file_too_large" || diagnostics[0].Path != "big.go" || !strings.Contains(diagnostics[0].Message, "100 bytes, exceeds max 50") {
		t.Errorf("baselineReadFailure = %v, %v; want one file_too_large diagnostic for big.go and no error", diagnostics, stalled)
	}
}

func TestBaselineReadFailureEndsTheScanWhenGitDoesNotAnswer(t *testing.T) {
	timedOut := boundedRead(t, func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "sleep", "30") }, 1<<10, 200*time.Millisecond)

	diagnostics, stalled := baselineReadFailure("a.go", timedOut)

	if stalled == nil || diagnostics != nil {
		t.Errorf("baselineReadFailure = %v, %v; want the operational error and no diagnostics", diagnostics, stalled)
	}
}

func TestBaselineReadFailureKeepsTheScanGoingForAFileGitCouldNotGive(t *testing.T) {
	diagnostics, stalled := baselineReadFailure("a.go", errors.New(`"a.go" is missing at abc`))

	if stalled != nil || len(diagnostics) != 1 || diagnostics[0].Kind != "head_read_failed" {
		t.Errorf("baselineReadFailure = %v, %v; want one head_read_failed diagnostic and no error", diagnostics, stalled)
	}
}
