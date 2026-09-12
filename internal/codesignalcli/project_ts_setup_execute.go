package codesignalcli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"sync"
)

// maxSetupExecutionOutput bounds the combined stdout+stderr ExecuteSetup
// captures from a setup command, so a misbehaving or unexpectedly verbose
// package manager cannot exhaust memory. Output beyond this bound is
// silently dropped rather than failing the run -- the command's own exit
// code is still authoritative.
const maxSetupExecutionOutput = 1 << 20

// ErrSetupExecutionNotConfirmed reports that ExecuteSetup was called without
// confirmed set (AC-SET-3): no subprocess is started.
var ErrSetupExecutionNotConfirmed = errors.New("setup execution: not confirmed")

// ErrSetupExecutionUnverifiedCommand reports that preview's executable and
// argv do not match one of the three frozen adapter rows exactly
// (SA-280-012): no subprocess is started. This covers both an executable
// outside {npm, pnpm, bun} and a preview whose Args were altered after
// BuildSetupPreview produced it -- ExecuteSetup never re-derives or trusts a
// command it did not itself verify against the frozen matrix.
var ErrSetupExecutionUnverifiedCommand = errors.New("setup execution: preview does not match a frozen adapter command")

// SetupExecutionResult is what actually ran, or would have run, for a
// confirmed setup command: the fields mirror SetupPreview's Executable/
// Args/WorkingDirectory unchanged, plus the outcome. Succeeded is true only
// on an exit-0 completion; a non-zero exit or a spawn failure leaves it
// false with ExitCode carrying whatever detail is available (-1 when no
// process exit code exists at all). Policy on how to react to a failed or
// timed-out run belongs to a later task -- ExecuteSetup only reports what
// happened.
type SetupExecutionResult struct {
	Executable       string
	Args             []string
	WorkingDirectory string
	ExitCode         int
	Succeeded        bool
	TimedOut         bool
	Output           []byte
}

// ExecuteSetup runs preview's exact command after a single explicit
// confirmation (AC-SET-3): it takes preview's Executable/Args/
// WorkingDirectory as-is, verifies them against the frozen adapter matrix
// (setupCommandTemplates) that produced them, and refuses -- spawning
// nothing -- if confirmed is false or that verification fails. It never
// rebuilds a shell command string and never invokes a shell: the child
// process is started directly via exec.CommandContext with argv set from
// preview.Args, so a value containing shell metacharacters can only ever
// reach the child as a literal, inert argument or working directory.
//
// The child runs with an explicit, minimal environment (PATH and HOME
// only) rather than this process's ambient environment, so a
// repository-controlled variable (for example an npm/pnpm registry
// override) cannot influence the run underneath the displayed command.
func ExecuteSetup(ctx context.Context, preview SetupPreview, confirmed bool) (SetupExecutionResult, error) {
	if !confirmed {
		return SetupExecutionResult{}, ErrSetupExecutionNotConfirmed
	}
	template, ok := setupCommandTemplates[preview.Executable]
	if !ok || !slices.Equal(preview.Args, template.args) {
		return SetupExecutionResult{}, fmt.Errorf("%w: executable %q args %v", ErrSetupExecutionUnverifiedCommand, preview.Executable, preview.Args)
	}

	timeout := preview.Timeout
	if timeout <= 0 {
		timeout = SetupPreviewTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, preview.Executable, preview.Args...)
	cmd.Dir = preview.WorkingDirectory
	cmd.Env = setupExecutionEnv()

	output := &boundedOutputSink{limit: maxSetupExecutionOutput}
	cmd.Stdout = output
	cmd.Stderr = output

	result := SetupExecutionResult{
		Executable:       preview.Executable,
		Args:             preview.Args,
		WorkingDirectory: preview.WorkingDirectory,
	}

	if startErr := cmd.Start(); startErr != nil {
		result.ExitCode = -1
		result.Output = output.Bytes()
		return result, nil
	}

	waitErr := cmd.Wait()
	result.Output = output.Bytes()

	// Checked ahead of waitErr: killing the child on deadline makes waitErr
	// non-nil too, but the deadline is the true, deterministic cause.
	if runCtx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		result.ExitCode = -1
		return result, nil
	}
	if waitErr == nil {
		result.Succeeded = true
		return result, nil
	}
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}
	result.ExitCode = -1
	return result, nil
}

// setupExecutionEnv is the child process's entire environment: PATH so the
// package manager can resolve itself and node, and HOME for its config/cache
// directories. Nothing else is forwarded, so a repository-controlled or
// otherwise ambient variable (npm_config_*, PNPM_*, registry overrides, a
// re-enabled lifecycle-script setting) can never reach the child -- this
// mirrors project_ts_compiler_mise_probe.go's confinement pattern.
func setupExecutionEnv() []string {
	env := []string{"PATH=" + os.Getenv("PATH")}
	if home := os.Getenv("HOME"); home != "" {
		env = append(env, "HOME="+home)
	}
	return env
}

// boundedOutputSink is an io.Writer that keeps at most limit bytes,
// silently discarding anything past that bound. It is safe for concurrent
// use because os/exec copies a Cmd's Stdout and Stderr pipes on separate
// goroutines whenever they are not the same *os.File.
type boundedOutputSink struct {
	mu    sync.Mutex
	buf   bytes.Buffer
	limit int
}

func (s *boundedOutputSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if remaining := s.limit - s.buf.Len(); remaining > 0 {
		if len(p) > remaining {
			s.buf.Write(p[:remaining])
		} else {
			s.buf.Write(p)
		}
	}
	return len(p), nil
}

func (s *boundedOutputSink) Bytes() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.buf.Bytes()...)
}
