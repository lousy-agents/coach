package codesignalcli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// errBoundedProbeTimedOut is runBoundedSubprocessProbe's shared timeout
// signal; every host subprocess probe in this package (Node/mise version
// and install-location detection) maps it onto its own domain error rather
// than leaking a generic message to a readiness/runtime consumer.
var errBoundedProbeTimedOut = errors.New("bounded subprocess probe timed out")

// runBoundedSubprocessProbe runs name with args, capped at timeout wall
// clock and maxOutput bytes of stdout, so a hung or unbounded-output host
// tool fails closed rather than wedging the CLI. exitErr carries the
// process's own non-zero-exit error uninterpreted -- callers assign their
// own meaning (a non-zero mise exit means "no candidate"; a non-zero node
// exit means a real error) -- while err covers every other spawn/read
// failure, with errBoundedProbeTimedOut identifying a deadline specifically.
func runBoundedSubprocessProbe(ctx context.Context, timeout time.Duration, maxOutput int64, name string, args ...string) (data []byte, exitErr, err error) {
	return runBoundedSubprocessProbeAt(ctx, timeout, maxOutput, "", nil, name, args...)
}

func runBoundedSubprocessProbeAt(ctx context.Context, timeout time.Duration, maxOutput int64, dir string, env []string, name string, args ...string) (data []byte, exitErr, err error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if env != nil {
		cmd.Env = env
	}
	stdout, pipeErr := cmd.StdoutPipe()
	if pipeErr != nil {
		return nil, nil, fmt.Errorf("starting %s: %w", name, pipeErr)
	}
	if startErr := cmd.Start(); startErr != nil {
		return nil, nil, fmt.Errorf("starting %s: %w", name, startErr)
	}

	data, readErr := io.ReadAll(io.LimitReader(stdout, maxOutput+1))
	waitErr := cmd.Wait()

	// Checked ahead of readErr/waitErr: killing the child on deadline makes
	// both of those non-nil too, but the deadline is the true, deterministic
	// cause and must classify as a timeout rather than a generic run/read
	// failure.
	if ctx.Err() == context.DeadlineExceeded {
		return nil, nil, errBoundedProbeTimedOut
	}
	if readErr != nil {
		return nil, nil, fmt.Errorf("reading %s output: %w", name, readErr)
	}
	if int64(len(data)) > maxOutput {
		return nil, nil, fmt.Errorf("%s output exceeded %d-byte budget", name, maxOutput)
	}
	if waitErr != nil {
		return nil, waitErr, nil
	}
	return data, nil, nil
}
