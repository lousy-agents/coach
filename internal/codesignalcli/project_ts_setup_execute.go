package codesignalcli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"
)

// maxSetupExecutionOutput bounds the combined stdout+stderr ExecuteSetup
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

// ErrSetupExecutionNotConfirmed reports that ExecuteSetup was called without
// confirmed set (AC-SET-3): no subprocess is started.
var ErrSetupExecutionNotConfirmed = errors.New("setup execution: not confirmed")

// ErrSetupExecutionUnverifiedCommand reports that preview's executable,
// argv, or timeout do not match one of the three frozen adapter rows
// exactly (SA-280-012): no subprocess is started. This covers an executable
// outside {npm, pnpm, bun}, a preview whose Args or Timeout were altered
// after BuildSetupPreview produced it, and a manager kind whose adapter row
// executable does not equal the kind's own name -- ExecuteSetup never
// re-derives or trusts a command it did not itself verify against the
// frozen matrix.
var ErrSetupExecutionUnverifiedCommand = errors.New("setup execution: preview does not match a frozen adapter command")

// SetupExecutionResult is what actually ran, or would have run, for a
// confirmed setup command: the fields mirror SetupPreview's Executable/
// Args/WorkingDirectory unchanged, plus the outcome. Succeeded is true only
// on an exit-0 completion; a non-zero exit or a spawn failure leaves it
// false with ExitCode carrying whatever detail is available (-1 when no
// process exit code exists at all). This exit status is recovered from
// cmd.ProcessState even when cmd.Wait itself returned exec.ErrWaitDelay
// because a surviving descendant (not the direct child) kept the output
// pipe open past setupExecutionWaitDelay: the direct child's own exit code
// is not in doubt in that case, only how long its output goroutines took to
// notice the pipe was closed. Policy on how to react to a failed or
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
// WorkingDirectory/Timeout as-is, verifies them against the frozen adapter
// matrix (setupCommandTemplates) that produced them -- executable, kind,
// argv, and timeout all must match exactly -- and refuses -- spawning
// nothing -- if confirmed is false or that verification fails. It never
// rebuilds a shell command string and never invokes a shell: the child
// process is started directly via exec.CommandContext with argv set from
// preview.Args, so a value containing shell metacharacters can only ever
// reach the child as a literal, inert argument or working directory.
//
// The child runs with an explicit, minimal environment (PATH and HOME
// only) rather than this process's ambient environment, so a
// repository-controlled or otherwise ambient environment variable (for
// example an npm/pnpm registry override) cannot influence the run
// underneath the displayed command. This says nothing about
// preview.WorkingDirectory's on-disk package-manager configuration (.npmrc,
// .pnpmfile.cjs, bunfig.toml, pnpm-workspace.yaml): those are read from the
// working directory regardless of environment confinement. ExecuteSetup
// assumes that configuration was already hazard-checked by
// checkPackageManager before this choice was ever offered to the caller --
// today that check is npm-only and rooted at the repository's worktree
// root, not at an arbitrary WorkingDirectory inside a monorepo, so a
// package-level .npmrc in a monorepo subdirectory is not covered by it.
func ExecuteSetup(ctx context.Context, preview SetupPreview, confirmed bool) (SetupExecutionResult, error) {
	if !confirmed {
		return SetupExecutionResult{}, ErrSetupExecutionNotConfirmed
	}
	template, ok := setupCommandTemplates[preview.Executable]
	if !ok || template.executable != preview.Executable || !slices.Equal(preview.Args, template.args) || preview.Timeout != SetupPreviewTimeout {
		return SetupExecutionResult{}, fmt.Errorf("%w: executable %q args %v timeout %s", ErrSetupExecutionUnverifiedCommand, preview.Executable, preview.Args, preview.Timeout)
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
	// would incorrectly trip a later task's failure-handling path).
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

// SetupOutcomeKind classifies the caller-facing policy decision
// RunConfirmedSetup returns. SetupOutcomeCancelled and SetupOutcomeFailed
// both mean the caller must exit 2 and emit no CodeSignal report (AC-SET-7,
// AC-SET-8's cancellation clause); SetupOutcomeSucceeded means the caller
// may proceed -- a later task's post-install readiness recheck decides what
// happens next.
type SetupOutcomeKind int

const (
	SetupOutcomeCancelled SetupOutcomeKind = iota
	SetupOutcomeFailed
	SetupOutcomeSucceeded
)

// SetupOutcome is RunConfirmedSetup's result: the caller-facing policy
// decision (Kind, ExitCode), plus enough detail to act on it. Execution is
// the zero SetupExecutionResult when Kind is SetupOutcomeCancelled, since
// ExecuteSetup is never called in that case. ChangedPaths is populated only
// when Kind is SetupOutcomeFailed, identifying files that may have changed
// (AC-SET-7) -- nothing here attempts to undo them.
type SetupOutcome struct {
	Kind         SetupOutcomeKind
	ExitCode     int
	Execution    SetupExecutionResult
	ChangedPaths []string
}

// RunConfirmedSetup translates a single confirmation decision into the
// exit-2/no-report policy a caller (a future CLI layer) must act on. It
// never attempts a rollback or any other cleanup of a failed or cancelled
// run (AC-SET-7, AC-18): the only remediation offered is disclosure of what
// may have changed.
//
// When confirmed is false, RunConfirmedSetup returns SetupOutcomeCancelled
// without calling ExecuteSetup at all. Refusing here -- rather than relying
// on ExecuteSetup's own confirmation guard to also refuse -- is what makes
// "cancelled, exit 2, no report" a policy decision this function owns, not
// an accident of what ExecuteSetup happens to also do.
//
// When confirmed is true, RunConfirmedSetup calls ExecuteSetup and
// classifies the result: a verification/spawn error, or a completed run
// that did not succeed (non-zero exit or timeout), both become
// SetupOutcomeFailed, with ChangedPaths populated from a best-effort read of
// preview.WorkingDirectory. A successful run becomes SetupOutcomeSucceeded.
func RunConfirmedSetup(ctx context.Context, preview SetupPreview, confirmed bool) (SetupOutcome, error) {
	if !confirmed {
		return SetupOutcome{Kind: SetupOutcomeCancelled, ExitCode: 2}, nil
	}

	execution, err := ExecuteSetup(ctx, preview, confirmed)
	if err != nil {
		return SetupOutcome{
			Kind:         SetupOutcomeFailed,
			ExitCode:     2,
			Execution:    execution,
			ChangedPaths: setupResidueChangedPaths(preview.WorkingDirectory),
		}, err
	}
	if !execution.Succeeded {
		return SetupOutcome{
			Kind:         SetupOutcomeFailed,
			ExitCode:     2,
			Execution:    execution,
			ChangedPaths: setupResidueChangedPaths(preview.WorkingDirectory),
		}, nil
	}
	return SetupOutcome{Kind: SetupOutcomeSucceeded, Execution: execution}, nil
}

// Bounds for setupResidueChangedPaths' read-only `git status` call: a small
// timeout and small output caps, since this is a status listing for a single
// working directory, not a whole-tree read.
const (
	setupResidueGitTimeout   = 10 * time.Second
	maxSetupResidueGitBytes  = 1 << 20 // 1 MiB
	maxSetupResidueGitStderr = 64 << 10
)

// setupResidueChangedPaths returns the untracked, modified, and gitignored
// paths (`--ignored`, since a package-manager install typically leaves a
// gitignored node_modules/ partially populated) that `git status --porcelain`
// reports for workingDirectory, for AC-SET-7's "identify files that may have
// changed" disclosure after a failed setup. This only ever reads: it never
// invokes `git reset`/`git clean`/`git checkout` or any other command that
// could mutate workingDirectory.
//
// It is best-effort and fails closed toward disclosure, not toward silence:
// if workingDirectory is not inside a Git worktree, or the bounded git
// status call otherwise fails, it reports workingDirectory itself as the one
// path a caller should inspect, rather than claiming nothing changed.
func setupResidueChangedPaths(workingDirectory string) []string {
	output, err := runGitBytesBounded(workingDirectory, maxSetupResidueGitBytes, maxSetupResidueGitStderr, setupResidueGitTimeout, "status", "--porcelain", "--ignored")
	if err != nil {
		return []string{workingDirectory}
	}
	return parseSetupResidueStatusPaths(output)
}

// parseSetupResidueStatusPaths extracts the path from each
// `git status --porcelain` line ("XY<space><path>", XY being two status
// characters), returning nil rather than an empty non-nil slice when there
// is nothing to report.
func parseSetupResidueStatusPaths(output []byte) []string {
	var paths []string
	for _, line := range strings.Split(strings.TrimRight(string(output), "\n"), "\n") {
		if len(line) < 4 {
			continue
		}
		paths = append(paths, strings.TrimSpace(line[3:]))
	}
	return paths
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
// silently discarding anything past that bound. ExecuteSetup assigns the
// same *boundedOutputSink to both cmd.Stdout and cmd.Stderr; os/exec
// detects that the two writers are identical (via interfaceEqual, not
// identical-*os.File) and collapses them onto a single pipe read by a
// single copier goroutine, so in practice Write is never called
// concurrently here. The mutex guards Bytes() against a future change that
// gives Stdout and Stderr distinct writers, which would restore concurrent
// Writes.
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
