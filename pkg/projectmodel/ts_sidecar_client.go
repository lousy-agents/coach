package projectmodel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
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

func tsSidecarTransportFailure(
	ctx, runCtx context.Context,
	timeout time.Duration,
	oversized bool,
	readErr, waitErr error,
	stderr *boundedWriter,
) string {
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		if timeout > 0 {
			return fmt.Sprintf("ts sidecar timed out after %s", timeout)
		}
		return "ts sidecar timed out (caller deadline exceeded)"
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return "ts sidecar canceled"
	}
	if oversized {
		return fmt.Sprintf("ts sidecar response exceeded %d-byte budget", maxTSSidecarResponseBytes)
	}
	if readErr != nil {
		return fmt.Sprintf("reading ts sidecar output: %s", readErr)
	}
	if waitErr != nil {
		if tail := strings.TrimSpace(stderr.buf.String()); tail != "" {
			return fmt.Sprintf("ts sidecar exited: %s: %s", waitErr, tail)
		}
		return fmt.Sprintf("ts sidecar exited: %s", waitErr)
	}
	return ""
}

func decodeTSSidecarResponse(data []byte, reqID int64) (projectbridge.Response, string) {
	var resp projectbridge.Response
	if err := json.Unmarshal(bytes.TrimRight(firstLine(data), "\n"), &resp); err != nil {
		return projectbridge.Response{}, fmt.Sprintf("ts sidecar produced malformed response: %s", err)
	}
	if resp.Version != projectbridge.ProtocolVersion || resp.ID != reqID {
		return projectbridge.Response{}, fmt.Sprintf("ts sidecar response protocol mismatch: version %d id %d", resp.Version, resp.ID)
	}
	return resp, ""
}

// sanitizedTSSidecarEnv is the minimal environment for the sidecar child,
// mirroring internal/codesignalcli/project_snapshot.go's
// sanitizedSnapshotGitEnv: only PATH and HOME are forwarded so the
// sidecar can locate itself and any runtime it wraps (e.g. a Node
// installation), never the parent process's full ambient environment.
func sanitizedTSSidecarEnv() []string {
	var env []string
	if value, ok := os.LookupEnv("PATH"); ok {
		env = append(env, "PATH="+value)
	}
	if value, ok := os.LookupEnv("HOME"); ok {
		env = append(env, "HOME="+value)
	}
	return env
}

// boundedWriter retains only the first limit bytes written to it, silently
// discarding the rest, so an untrusted child's stderr cannot grow the
// diagnostic message without bound.
type boundedWriter struct {
	buf   bytes.Buffer
	limit int
}

func (w *boundedWriter) Write(p []byte) (int, error) {
	if room := w.limit - w.buf.Len(); room > 0 {
		if room > len(p) {
			room = len(p)
		}
		w.buf.Write(p[:room])
	}
	return len(p), nil
}

func firstLine(data []byte) []byte {
	if idx := bytes.IndexByte(data, '\n'); idx >= 0 {
		return data[:idx]
	}
	return data
}
