package projectmodel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

func tsSidecarTransportFailure(
	ctx, runCtx context.Context,
	timeout time.Duration,
	oversized bool,
	readErr, waitErr error,
	stderr *boundedWriter,
) string {
	if msg := tsSidecarTimeoutMessage(ctx, runCtx, timeout); msg != "" {
		return msg
	}
	if oversized {
		return fmt.Sprintf("ts sidecar response exceeded %d-byte budget", maxTSSidecarResponseBytes)
	}
	if readErr != nil {
		return fmt.Sprintf("reading ts sidecar output: %s", readErr)
	}
	return tsSidecarWaitFailure(waitErr, stderr)
}

func tsSidecarTimeoutMessage(ctx, runCtx context.Context, timeout time.Duration) string {
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

func tsSidecarWaitFailure(waitErr error, stderr *boundedWriter) string {
	if waitErr == nil {
		return ""
	}
	if tail := strings.TrimSpace(stderr.buf.String()); tail != "" {
		return fmt.Sprintf("ts sidecar exited: %s: %s", waitErr, tail)
	}
	return fmt.Sprintf("ts sidecar exited: %s", waitErr)
}
