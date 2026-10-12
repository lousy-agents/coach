package gitrepo

import (
	"context"
	"os/exec"
	"time"
)

// BoundErrorKind distinguishes which of RunBytesBoundedWith's
// own bounds tripped, so a caller can classify a size-budget failure
// (customer-controlled content) differently from a timeout or stderr-budget
// failure (a resource/environment condition, not content the config author
// can shrink by hand).
type BoundErrorKind int

const (
	BoundTimeout BoundErrorKind = iota
	BoundStdout
	BoundStderr
)

// BoundError marks a RunBytesBoundedWith failure that comes
// from our own timeout/output-budget enforcement rather than from git's
// stderr. Its message is already complete and safe to surface verbatim to a
// --project-config user: unlike a git failure, it never embeds raw git
// stderr.
type BoundError struct {
	message string
	Kind    BoundErrorKind
}

// commandContext builds the git child used by bounded reads. Tests may
// replace it to simulate hung or oversized children.
var commandContext = func(ctx context.Context, dir string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
}

// RunBytesBoundedWith is the shared bounded-git-read implementation
// behind RunBytesBounded: a wall-time limit, hard stdout/stderr caps, and
// concurrent pipe draining. buildCmd is the child-construction seam, letting
// callers vary command/environment construction (e.g. revisionfs's
// sanitized-environment snapshot reads) without duplicating this I/O logic.
func RunBytesBoundedWith(buildCmd func(ctx context.Context, dir string, args ...string) *exec.Cmd, dir string, maxStdout, maxStderr int64, timeout time.Duration, args ...string) ([]byte, error) {
	if err := validateGitReadBounds(timeout, maxStdout, maxStderr); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := buildCmd(ctx, dir, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Drain stdout and stderr concurrently. Sequential reads deadlock when
	// the child fills one pipe's OS buffer while the parent is still
	// blocked reading the other.
	stdoutCh := make(chan gitPipeResult, 1)
	stderrCh := make(chan gitPipeResult, 1)
	go func() {
		stdoutCh <- readGitPipe(stdout, maxStdout+1)
	}()
	go func() {
		stderrCh <- readGitPipe(stderr, maxStderr+1)
	}()
	return finishGitBoundedRead(<-stdoutCh, <-stderrCh, cmd.Wait(), ctx, maxStdout, maxStderr, timeout)
}

// RunBytesBounded runs git with a wall-time limit and hard caps on
// collected stdout and stderr, building the child via the package's default
// commandContext seam. The LimitReader stops after maxStdout+1 bytes so
// an oversized blob is detected without buffering the entire child output.
func RunBytesBounded(dir string, maxStdout, maxStderr int64, timeout time.Duration, args ...string) ([]byte, error) {
	return RunBytesBoundedWith(commandContext, dir, maxStdout, maxStderr, timeout, args...)
}

func (e *BoundError) Error() string { return e.message }
