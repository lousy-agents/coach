package tssidecar

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

func transportFailure(
	ctx, runCtx context.Context,
	timeout time.Duration,
	oversized bool,
	readErr, waitErr error,
	stderr *boundedWriter,
) string {
	if msg := timeoutMessage(ctx, runCtx, timeout); msg != "" {
		return msg
	}
	if oversized {
		return fmt.Sprintf("ts sidecar response exceeded %d-byte budget", maxResponseBytes)
	}
	if readErr != nil {
		return fmt.Sprintf("reading ts sidecar output: %s", readErr)
	}
	return waitFailure(waitErr, stderr)
}

func timeoutMessage(ctx, runCtx context.Context, timeout time.Duration) string {
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		if timeout > 0 {
			return fmt.Sprintf("ts sidecar timed out after %s", timeout)
		}
		return "ts sidecar timed out (caller deadline exceeded)"
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return "ts sidecar canceled"
	}
	return ""
}

func waitFailure(waitErr error, stderr *boundedWriter) string {
	if waitErr == nil {
		return ""
	}
	if tail := scrubStderr(stderr.buf.String()); tail != "" {
		return fmt.Sprintf("ts sidecar exited: %s: %s", waitErr, tail)
	}
	return fmt.Sprintf("ts sidecar exited: %s", waitErr)
}

func scrubStderr(raw string) string {
	var kept []string
	for _, line := range strings.Split(raw, "\n") {
		if strings.Contains(line, "file://") || strings.Contains(line, "node:internal") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}
