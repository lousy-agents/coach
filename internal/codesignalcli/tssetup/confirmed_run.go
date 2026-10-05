package tssetup

import (
	"context"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectcheck"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// OutcomeKind classifies the caller-facing policy decision
// RunConfirmed returns. OutcomeUnknown is the zero value and is
// never returned by RunConfirmed: it exists so a zero-initialized
// Outcome (for example a variable declared but not yet assigned) cannot
// be misread as OutcomeCancelled, whose own zero-valued ExitCode would
// otherwise look identical to "cancelled with exit code 0" instead of "not
// yet a real outcome". OutcomeCancelled and OutcomeFailed both
// mean the caller must exit 2 and emit no CodeSignal report (AC-SET-7,
// AC-SET-8's cancellation clause); OutcomeSucceeded means the caller
// may proceed -- RunConfirmedAndRecheckReadiness's post-install
// readiness recheck decides what happens next.
type OutcomeKind int

const (
	OutcomeUnknown OutcomeKind = iota
	OutcomeCancelled
	OutcomeFailed
	OutcomeSucceeded
)

// Outcome is RunConfirmed's result: the caller-facing policy
// decision (Kind, ExitCode), plus enough detail to act on it. Execution is
// the zero ExecutionResult both when Kind is OutcomeCancelled
// (Execute is never called) and when Execute itself refused to run
// (Kind is OutcomeFailed, but no subprocess ever started).
//
// ChangedPaths is populated only when Kind is OutcomeFailed *and* a
// subprocess actually ran, identifying files that may have changed
// (AC-SET-7) -- nothing here attempts to undo them. Its paths are
// repository-root-relative, not WorkingDirectory-relative (see
// setupResidueChangedPaths' doc comment), since they come from `git
// status`. ResidueUnknown is true when that disclosure itself could not be
// produced (WorkingDirectory is not inside a Git worktree, or the bounded
// status read otherwise failed): callers must treat that case as "Coach
// could not determine what changed", not as "nothing changed" or as an
// ordinary successful (if possibly empty) disclosure.
//
// PostInstallReadiness is populated only when Kind is OutcomeSucceeded,
// by RunConfirmedAndRecheckReadiness (AC-SET-6): it is nil on every
// other path, including a plain RunConfirmed call, since only that
// wrapper reruns readiness.
type Outcome struct {
	Kind                 OutcomeKind
	ExitCode             int
	Execution            ExecutionResult
	ChangedPaths         []string
	ResidueUnknown       bool
	PostInstallReadiness *projectreadiness.Result
}

// RunConfirmed translates a single confirmation decision into the
// exit-2/no-report policy a caller (a future CLI layer) must act on. It
// never attempts a rollback or any other cleanup of a failed or cancelled
// run (AC-SET-7, AC-18): the only remediation offered is disclosure of what
// may have changed.
//
// When confirmed is false, RunConfirmed returns OutcomeCancelled
// without calling Execute at all. Refusing here -- rather than relying
// on Execute's own confirmation guard to also refuse -- is what makes
// "cancelled, exit 2, no report" a policy decision this function owns, not
// an accident of what Execute happens to also do.
//
// When confirmed is true, RunConfirmed calls Execute and
// classifies the result. If Execute itself returns an error, no
// subprocess ever started (an unconfirmed call this function never actually
// makes, or a preview that failed frozen-matrix verification), so
// RunConfirmed reports OutcomeFailed without scanning for residue
// at all: a run that never began cannot have changed anything, and scanning
// anyway would misattribute whatever was already dirty in the worktree to
// it. If Execute runs but does not succeed (non-zero exit or timeout),
// RunConfirmed reports OutcomeFailed with ChangedPaths and
// ResidueUnknown populated from a best-effort, WorkingDirectory-scoped
// residue read (see ResidueUnknown's doc). A successful run becomes
// OutcomeSucceeded.
func RunConfirmed(ctx context.Context, preview Preview, confirmed bool) (Outcome, error) {
	if !confirmed {
		return Outcome{Kind: OutcomeCancelled, ExitCode: 2}, nil
	}

	execution, err := Execute(ctx, preview, confirmed)
	if err != nil {
		return Outcome{
			Kind:      OutcomeFailed,
			ExitCode:  2,
			Execution: execution,
		}, err
	}
	if !execution.Succeeded {
		changedPaths, residueUnknown := setupResidueChangedPaths(preview.WorkingDirectory)
		return Outcome{
			Kind:           OutcomeFailed,
			ExitCode:       2,
			Execution:      execution,
			ChangedPaths:   changedPaths,
			ResidueUnknown: residueUnknown,
		}, nil
	}
	return Outcome{Kind: OutcomeSucceeded, Execution: execution}, nil
}

// RunConfirmedAndRecheckReadiness runs RunConfirmed and, only when
// it succeeds, reruns the complete readiness check via projectcheck.Run
// (AC-SET-6) using the same dir/revision/configPath that produced the stale
// readiness result which offered this setup in the first place, attaching
// the fresh result to the returned Outcome's PostInstallReadiness. A
// cancelled or failed outcome (and a RunConfirmed error) is returned
// unchanged, with PostInstallReadiness left nil: nothing was installed, so
// there is nothing new to recheck. If the recheck itself errors, the
// outcome's Kind still reflects the install's own success, but the error is
// returned and PostInstallReadiness is left nil -- a caller must treat that
// as "the recheck itself failed", not as "the install failed" or as a clean
// readiness result. This function does not interpret PostInstallReadiness; a
// caller decides whether the fresh result clears the gap that offered this
// setup.
func RunConfirmedAndRecheckReadiness(ctx context.Context, preview Preview, confirmed bool, dir, revision, configPath string) (Outcome, error) {
	outcome, err := RunConfirmed(ctx, preview, confirmed)
	if err != nil || outcome.Kind != OutcomeSucceeded {
		return outcome, err
	}
	readiness, readinessErr := projectcheck.Run(dir, revision, configPath)
	if readinessErr != nil {
		return outcome, readinessErr
	}
	outcome.PostInstallReadiness = readiness
	return outcome, nil
}

// setupExecutionEnv is the child process's entire environment: PATH so the
// package manager can resolve itself and node, and HOME for its config/cache
// directories. Nothing else is forwarded, so a repository-controlled or
// otherwise ambient variable (npm_config_*, PNPM_*, registry overrides, a
// re-enabled lifecycle-script setting) can never reach the child -- this
// mirrors tstoolchain's mise probe confinement pattern.
func setupExecutionEnv() []string {
	env := []string{"PATH=" + os.Getenv("PATH")}
	if home := os.Getenv("HOME"); home != "" {
		env = append(env, "HOME="+home)
	}
	return env
}
