// Package tssidecar runs the pinned Node/TypeScript project sidecar as a
// child process for one internal/projectbridge request. It is
// pkg/projectmodel's driven adapter for the TypeScript analyzer: it owns
// process spawning, the child's environment, output bounds, and the wording
// of every transport failure, and knows nothing about the project Model
// built from the response.
package tssidecar

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/lousy-agents/coach/internal/projectbridge"
)

const maxResponseBytes = 8 << 20 // 8 MiB

const maxStderrBytes = 4 << 10 // 4 KiB

// Process is one configured sidecar invocation: BinaryPath is started with
// Args in Dir, with Path as the child's PATH (see sanitizedEnv), and is
// bounded by Timeout when it is positive.
type Process struct {
	BinaryPath string
	Path       string
	Dir        string
	Args       []string
	Timeout    time.Duration
}

// Analyze runs the sidecar transport for one request: it stats the binary,
// spawns it with a minimal explicit environment and Args, writes req to its
// stdin, and reads one bounded response line from its stdout. On any
// transport failure it returns an error whose message describes which
// failure mode occurred (missing binary, failed start, non-zero exit,
// oversized output, or timeout); the caller turns that into a
// DiagBackendUnavailable diagnostic rather than a Go error.
func (p Process) Analyze(ctx context.Context, req projectbridge.Request) (projectbridge.Response, error) {
	resp, failure := p.run(ctx, req)
	if failure != "" {
		return projectbridge.Response{}, errors.New(failure)
	}
	return resp, nil
}

func (p Process) run(ctx context.Context, req projectbridge.Request) (projectbridge.Response, string) {
	if _, err := os.Stat(p.BinaryPath); err != nil {
		return projectbridge.Response{}, fmt.Sprintf("ts sidecar binary unavailable at %q: %s", p.BinaryPath, err)
	}

	runCtx, cancelRun, cancelTimeout := runContext(ctx, p.Timeout)
	defer cancelRun()
	if cancelTimeout != nil {
		defer cancelTimeout()
	}

	reqLine, err := json.Marshal(req)
	if err != nil {
		return projectbridge.Response{}, fmt.Sprintf("encoding ts sidecar request: %s", err)
	}

	cmd, stderr, startErr := p.start(runCtx, reqLine)
	if startErr != "" {
		return projectbridge.Response{}, startErr
	}

	data, readErr := io.ReadAll(io.LimitReader(cmd.stdout, maxResponseBytes+1))
	oversized := int64(len(data)) > maxResponseBytes
	// Only cancel before Wait when the child may still be writing past the
	// budget (oversized) or the read itself failed; canceling
	// unconditionally races exec.CommandContext's own ctx-watchdog against
	// a child that has already exited normally.
	if oversized || readErr != nil {
		cancelRun()
	}
	waitErr := cmd.wait()

	if msg := transportFailure(ctx, runCtx, p.Timeout, oversized, readErr, waitErr, stderr); msg != "" {
		return projectbridge.Response{}, msg
	}
	return decodeResponse(data, req.ID)
}

func runContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc, context.CancelFunc) {
	runCtx := ctx
	var cancelTimeout context.CancelFunc
	if timeout > 0 {
		runCtx, cancelTimeout = context.WithTimeout(runCtx, timeout)
	}
	runCtx, cancelRun := context.WithCancel(runCtx)
	return runCtx, cancelRun, cancelTimeout
}

type proc struct {
	wait   func() error
	stdout io.ReadCloser
}

func (p Process) start(runCtx context.Context, reqLine []byte) (*proc, *boundedWriter, string) {
	cmd := exec.CommandContext(runCtx, p.BinaryPath, p.Args...)
	cmd.Dir = p.Dir
	cmd.Env = sanitizedEnv(p.Path)
	cmd.Stdin = bytes.NewReader(append(reqLine, '\n'))
	stderr := &boundedWriter{limit: maxStderrBytes}
	cmd.Stderr = stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Sprintf("starting ts sidecar: %s", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Sprintf("starting ts sidecar: %s", err)
	}
	return &proc{wait: cmd.Wait, stdout: stdout}, stderr, ""
}
