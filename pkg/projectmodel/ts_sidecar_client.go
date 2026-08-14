package projectmodel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/lousy-agents/coach/internal/projectbridge"
)

// callTSSidecar runs the sidecar transport for one request: it stats the
// binary, spawns it with a minimal explicit environment and opts.Args,
// writes req to its stdin, and reads one bounded response line from its
// stdout. On any transport failure it returns a non-empty human-readable
// message describing which failure mode occurred (missing binary, failed
// start, non-zero exit, oversized output, or timeout); the caller turns
// that into a DiagBackendUnavailable diagnostic rather than a Go error.
func callTSSidecar(ctx context.Context, opts TSSidecarOptions, req projectbridge.Request) (projectbridge.Response, string) {
	if _, err := os.Stat(opts.BinaryPath); err != nil {
		return projectbridge.Response{}, fmt.Sprintf("ts sidecar binary unavailable at %q: %s", opts.BinaryPath, err)
	}

	runCtx, cancelRun, cancelTimeout := tsSidecarRunContext(ctx, opts.Timeout)
	defer cancelRun()
	if cancelTimeout != nil {
		defer cancelTimeout()
	}

	reqLine, err := json.Marshal(req)
	if err != nil {
		return projectbridge.Response{}, fmt.Sprintf("encoding ts sidecar request: %s", err)
	}

	cmd, stderr, startErr := startTSSidecarCmd(runCtx, opts, reqLine)
	if startErr != "" {
		return projectbridge.Response{}, startErr
	}

	data, readErr := io.ReadAll(io.LimitReader(cmd.stdout, maxTSSidecarResponseBytes+1))
	oversized := int64(len(data)) > maxTSSidecarResponseBytes
	// Only cancel before Wait when the child may still be writing past the
	// budget (oversized) or the read itself failed; canceling
	// unconditionally races exec.CommandContext's own ctx-watchdog against
	// a child that has already exited normally.
	if oversized || readErr != nil {
		cancelRun()
	}
	waitErr := cmd.wait()

	if msg := tsSidecarTransportFailure(ctx, runCtx, opts.Timeout, oversized, readErr, waitErr, stderr); msg != "" {
		return projectbridge.Response{}, msg
	}
	return decodeTSSidecarResponse(data, req.ID)
}

func tsSidecarRunContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc, context.CancelFunc) {
	runCtx := ctx
	var cancelTimeout context.CancelFunc
	if timeout > 0 {
		runCtx, cancelTimeout = context.WithTimeout(runCtx, timeout)
	}
	runCtx, cancelRun := context.WithCancel(runCtx)
	return runCtx, cancelRun, cancelTimeout
}

type tsSidecarProc struct {
	wait   func() error
	stdout io.ReadCloser
}

func startTSSidecarCmd(runCtx context.Context, opts TSSidecarOptions, reqLine []byte) (*tsSidecarProc, *boundedWriter, string) {
	cmd := exec.CommandContext(runCtx, opts.BinaryPath, opts.Args...)
	cmd.Env = sanitizedTSSidecarEnv()
	cmd.Stdin = bytes.NewReader(append(reqLine, '\n'))
	stderr := &boundedWriter{limit: maxTSSidecarStderrBytes}
	cmd.Stderr = stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Sprintf("starting ts sidecar: %s", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Sprintf("starting ts sidecar: %s", err)
	}
	return &tsSidecarProc{wait: cmd.Wait, stdout: stdout}, stderr, ""
}
