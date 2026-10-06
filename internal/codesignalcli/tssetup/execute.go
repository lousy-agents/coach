package tssetup

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/pkgmanager"
)

// maxSetupExecutionOutput bounds the combined stdout+stderr Execute
// captures from a setup command, so a misbehaving or unexpectedly verbose
// package manager cannot exhaust memory. Output beyond this bound is
// silently dropped rather than failing the run -- the command's own exit
// code is still authoritative.
const maxSetupExecutionOutput = 1 << 20

// setupExecutionWaitDelay bounds how long cmd.Wait may keep waiting for
// output-copying goroutines to finish after the run's deadline (or an
// external ctx cancellation) fires. exec.CommandContext only SIGKILLs the
// direct child; npm/pnpm/bun all spawn node children, and pnpm runs a
// background store server, any of which can outlive the direct child while
// still holding the inherited stdout/stderr pipe open. Without a WaitDelay,
// cmd.Wait blocks until every process sharing that pipe exits, which can be
// indefinitely -- so the "bounded timeout" this package promises would not
// actually be bounded.
const setupExecutionWaitDelay = 2 * time.Second

// ErrNotConfirmed reports that Execute was called without
// confirmed set (AC-SET-3): no subprocess is started.
var ErrNotConfirmed = errors.New("setup execution: not confirmed")

// ErrUnverifiedCommand reports that preview's executable,
// argv, or timeout do not match one of the three frozen adapter rows
// exactly (SA-280-012): no subprocess is started. This covers an executable
// outside {npm, pnpm, bun}, a preview whose Args or Timeout were altered
// after BuildPreview produced it, and a manager kind whose adapter row
// executable does not equal the kind's own name -- Execute never
// re-derives or trusts a command it did not itself verify against the
// frozen matrix.
var ErrUnverifiedCommand = errors.New("setup execution: preview does not match a frozen adapter command")

// ErrHazardousWorkingDirectory reports that
// preview.WorkingDirectory itself carries a package-manager configuration
// hazard (pkgmanager.DetectPackageManagerHazard) -- a committed .npmrc or bunfig.toml
// that would redirect the registry or otherwise bypass the frozen adapter's
// script suppression: no subprocess is started. pkgmanager.Check scans the
// selected roots' package contexts (pkgmanager.Contexts), which is not
// necessarily where a caller points preview.WorkingDirectory; Execute
// re-runs the same hazard scan directly against the directory it is about to
// run in, so the scan and the install can never be about different
// directories.
var ErrHazardousWorkingDirectory = errors.New("setup execution: working directory carries a package-manager configuration hazard")

// ExecutionResult is what actually ran, or would have run, for a
// confirmed setup command: the fields mirror Preview's Executable/
// Args/WorkingDirectory unchanged, plus the outcome. Succeeded is true only
// on an exit-0 completion; a non-zero exit or a spawn failure leaves it
// false with ExitCode carrying whatever detail is available (-1 when no
// process exit code exists at all). This exit status is recovered from
// cmd.ProcessState even when cmd.Wait itself returned exec.ErrWaitDelay
// because a surviving descendant (not the direct child) kept the output
// pipe open past setupExecutionWaitDelay: the direct child's own exit code
// is not in doubt in that case, only how long its output goroutines took to
// notice the pipe was closed. Policy on how to react to a failed or
// timed-out run belongs to RunConfirmed (below) -- Execute only
// reports what happened.
type ExecutionResult struct {
	Executable       string
	Args             []string
	WorkingDirectory string
	ExitCode         int
	Succeeded        bool
	TimedOut         bool
	Output           []byte
}

// Execute runs preview's exact command after a single explicit
// confirmation (AC-SET-3): it takes preview's Executable/Args/
// WorkingDirectory/Timeout as-is, verifies them against the frozen adapter
// matrix (setupCommandTemplates) that produced them -- executable, kind,
// argv, and timeout all must match exactly -- and refuses -- spawning
// nothing -- if confirmed is false or that verification fails. It never
// rebuilds a shell command string and never invokes a shell: the child
// process is started directly via exec.CommandContext with argv set from
// preview.Args, so a value containing shell metacharacters can only ever
// reach the child as a literal, inert argument or working directory.
//
// confirmed represents the caller already having obtained a single-use
// confirmation from the user before this call; Execute itself neither
// consumes nor invalidates that confirmation, so calling it twice with
// confirmed set both times runs the command twice. Enforcing single-use
// confirmation (preventing a caller from acting on the same confirmation
// more than once) is the interactive layer that collects it, not this
// library function.
//
// The child runs with an explicit, minimal environment (PATH and HOME
// only) rather than this process's ambient environment, so a
// repository-controlled or otherwise ambient environment variable (for
// example an npm/pnpm registry override) cannot influence the run
// underneath the displayed command. This says nothing about
// preview.WorkingDirectory's on-disk package-manager configuration files
// themselves (.npmrc, .pnpmfile.cjs, bunfig.toml, pnpm-workspace.yaml):
// those are read from the working directory regardless of environment
// confinement. Execute re-runs pkgmanager.DetectPackageManagerHazard directly
// against preview.WorkingDirectory (not only the selected roots' package
// contexts, which is what pkgmanager.Check scans when deciding whether to
// offer this choice at all) and refuses -- spawning nothing -- if it finds
// a hazard there (ErrHazardousWorkingDirectory), so a
// package-level .npmrc/bunfig.toml in a directory no selected root resolved
// to is covered too. .pnpmfile.cjs and pnpm-workspace.yaml are not scanned
// by either check.
func Execute(ctx context.Context, preview Preview, confirmed bool) (ExecutionResult, error) {
	if !confirmed {
		return ExecutionResult{}, ErrNotConfirmed
	}
	template, ok := setupCommandTemplates[preview.Executable]
	if !ok || template.executable != preview.Executable || !slices.Equal(preview.Args, template.args) || preview.Timeout != PreviewTimeout {
		return ExecutionResult{}, fmt.Errorf("%w: executable %q args %v timeout %s", ErrUnverifiedCommand, preview.Executable, preview.Args, preview.Timeout)
	}
	if detail := pkgmanager.DetectPackageManagerHazard(preview.WorkingDirectory, preview.Executable); detail != "" {
		return ExecutionResult{}, fmt.Errorf("%w: %s", ErrHazardousWorkingDirectory, detail)
	}

	runCtx, cancel := context.WithTimeout(ctx, preview.Timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, preview.Executable, preview.Args...)
	cmd.Dir = preview.WorkingDirectory
	cmd.Env = setupExecutionEnv()
	cmd.WaitDelay = setupExecutionWaitDelay

	output := &boundedOutputSink{limit: maxSetupExecutionOutput}
	cmd.Stdout = output
	cmd.Stderr = output

	result := ExecutionResult{
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
	// exec.ErrWaitDelay: the direct child already exited on its own (a
	// successful npm/pnpm/bun run can leave a store-server or
	// update-notifier descendant behind, still holding the inherited
	// stdout/stderr pipe open) and setupExecutionWaitDelay elapsed before the
	// output-copying goroutines finished, not before the process itself
	// finished. Per os/exec's Wait, cmd.ProcessState is populated from
	// cmd.Process.Wait() before the WaitDelay race even begins, so it still
	// carries the child's real exit status here -- recovering it, rather
	// than reporting ErrWaitDelay as a bare failure, is what keeps a
	// genuinely successful install from being misreported as failed (which
	// would incorrectly trip RunConfirmed's failure-handling path).
	if errors.Is(waitErr, exec.ErrWaitDelay) {
		if cmd.ProcessState != nil && cmd.ProcessState.Success() {
			result.Succeeded = true
			result.ExitCode = 0
			return result, nil
		}
		if cmd.ProcessState != nil {
			result.ExitCode = cmd.ProcessState.ExitCode()
			return result, nil
		}
		result.ExitCode = -1
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
