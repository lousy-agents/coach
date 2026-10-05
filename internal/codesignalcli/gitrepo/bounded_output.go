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

func finishGitBoundedRead(stdout, stderr gitPipeResult, waitErr error, ctx context.Context, maxStdout, maxStderr int64, timeout time.Duration) ([]byte, error) {
	if stdout.err != nil {
		return nil, stdout.err
	}
	if stderr.err != nil {
		return nil, stderr.err
	}
	if int64(len(stdout.data)) > maxStdout {
		return nil, &BoundError{Kind: BoundStdout, message: fmt.Sprintf("git stdout exceeded %d-byte budget", maxStdout)}
	}
	if int64(len(stderr.data)) > maxStderr {
		return nil, &BoundError{Kind: BoundStderr, message: fmt.Sprintf("git stderr exceeded %d-byte budget", maxStderr)}
	}
	if ctx.Err() == context.DeadlineExceeded {
		return nil, &BoundError{Kind: BoundTimeout, message: fmt.Sprintf("git execution timed out after %s", timeout)}
	}
	if waitErr != nil {
		return nil, gitWaitError(waitErr, stderr.data)
	}
	return stdout.data, nil
}

func gitWaitError(waitErr error, stderr []byte) error {
	if len(stderr) > 0 {
		return fmt.Errorf("%s: %s", waitErr, strings.TrimSpace(string(stderr)))
	}
	return waitErr
}
