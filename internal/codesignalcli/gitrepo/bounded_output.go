package gitrepo

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

type gitPipeResult struct {
	data []byte
	err  error
}

func validateGitReadBounds(timeout time.Duration, maxStdout, maxStderr int64) error {
	if timeout <= 0 {
		return fmt.Errorf("git execution timeout must be positive")
	}
	if maxStdout < 0 || maxStderr < 0 {
		return fmt.Errorf("git output bounds must be non-negative")
	}
	return nil
}

func readGitPipe(r io.Reader, limit int64) gitPipeResult {
	data, err := io.ReadAll(io.LimitReader(r, limit))
	return gitPipeResult{data: data, err: err}
}

// readGitPipeUntilOverrun reads at most limit+1 bytes and ends the git process the moment the
// stream proves to be over its budget. Past the budget nothing reads the pipe, so git blocks
// writing to it and would otherwise live until the wall-time bound killed it.
func readGitPipeUntilOverrun(r io.Reader, limit int64, endGit context.CancelFunc) gitPipeResult {
	result := readGitPipe(r, limit+1)
	if int64(len(result.data)) > limit {
		endGit()
	}
	return result
}

func finishGitBoundedRead(stdout, stderr gitPipeResult, waitErr error, ctx context.Context, maxStdout, maxStderr int64, timeout time.Duration) ([]byte, error) {
	// The bounds come before a read error: a read that was closed under, or cut short, because a bound ended
	// the read has failed because of that bound.
	if int64(len(stdout.data)) > maxStdout {
		return nil, &BoundError{Kind: BoundStdout, message: fmt.Sprintf("git stdout exceeded %d-byte budget", maxStdout)}
	}
	if int64(len(stderr.data)) > maxStderr {
		return nil, &BoundError{Kind: BoundStderr, message: fmt.Sprintf("git stderr exceeded %d-byte budget", maxStderr)}
	}
	if ctx.Err() == context.DeadlineExceeded {
		return nil, &BoundError{Kind: BoundTimeout, message: fmt.Sprintf("git execution timed out after %s", timeout)}
	}
	if stdout.err != nil {
		return nil, stdout.err
	}
	if stderr.err != nil {
		return nil, stderr.err
	}
	if waitErr != nil {
		return nil, gitWaitError(waitErr, stderr.data)
	}
	return stdout.data, nil
}

func gitWaitError(waitErr error, stderr []byte) error {
	if len(stderr) > 0 {
		return fmt.Errorf("%s: %s", waitErr, TerminalSafe(strings.TrimSpace(string(stderr))))
	}
	return waitErr
}
