package codesignalcli

import (
	"bytes"
	"context"

	"os"
)

// parseSetupResidueStatusPaths extracts the path from each NUL-delimited
// `git status --porcelain -z` record ("XY<space><path>\0", XY being two
// status characters). Unlike the newline-delimited "--porcelain" format
// alone, -z never C-quotes or octal-escapes a path, so a path containing a
// space, non-ASCII byte, or literal quote character survives unmodified. A
// rename or copy record (status 'R' or 'C' in either column) emits the
// origin path as an additional NUL-delimited field immediately after the
// status/path field; that field is the path *before* the change, so it is
// consumed and discarded here -- only the resulting path is a "may have
// changed" location worth disclosing. Returns nil rather than an empty
// non-nil slice when there is nothing to report.
func parseSetupResidueStatusPaths(output []byte) []string {
	fields := bytes.Split(bytes.TrimRight(output, "\x00"), []byte{0})
	if len(fields) == 1 && len(fields[0]) == 0 {
		return nil
	}
	var paths []string
	for i := 0; i < len(fields); i++ {
		entry := fields[i]
		if len(entry) < 4 {
			continue
		}
		paths = append(paths, string(entry[3:]))
		if entry[0] == 'R' || entry[0] == 'C' || entry[1] == 'R' || entry[1] == 'C' {
			i++
		}
	}
	return paths
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
// classifies the result. If ExecuteSetup itself returns an error, no
// subprocess ever started (an unconfirmed call this function never actually
// makes, or a preview that failed frozen-matrix verification), so
// RunConfirmedSetup reports SetupOutcomeFailed without scanning for residue
// at all: a run that never began cannot have changed anything, and scanning
// anyway would misattribute whatever was already dirty in the worktree to
// it. If ExecuteSetup runs but does not succeed (non-zero exit or timeout),
// RunConfirmedSetup reports SetupOutcomeFailed with ChangedPaths and
// ResidueUnknown populated from a best-effort, WorkingDirectory-scoped
// residue read (see ResidueUnknown's doc). A successful run becomes
// SetupOutcomeSucceeded.
func RunConfirmedSetup(ctx context.Context, preview SetupPreview, confirmed bool) (SetupOutcome, error) {
	if !confirmed {
		return SetupOutcome{Kind: SetupOutcomeCancelled, ExitCode: 2}, nil
	}

	execution, err := ExecuteSetup(ctx, preview, confirmed)
	if err != nil {
		return SetupOutcome{
			Kind:      SetupOutcomeFailed,
			ExitCode:  2,
			Execution: execution,
		}, err
	}
	if !execution.Succeeded {
		changedPaths, residueUnknown := setupResidueChangedPaths(preview.WorkingDirectory)
		return SetupOutcome{
			Kind:           SetupOutcomeFailed,
			ExitCode:       2,
			Execution:      execution,
			ChangedPaths:   changedPaths,
			ResidueUnknown: residueUnknown,
		}, nil
	}
	return SetupOutcome{Kind: SetupOutcomeSucceeded, Execution: execution}, nil
}

// RunConfirmedSetupAndRecheckReadiness runs RunConfirmedSetup and, only when
// it succeeds, reruns the complete readiness check via CheckProjectReadiness
// (AC-SET-6) using the same dir/revision/configPath that produced the stale
// readiness result which offered this setup in the first place, attaching
// the fresh result to the returned SetupOutcome's PostInstallReadiness. A
// cancelled or failed outcome (and a RunConfirmedSetup error) is returned
// unchanged, with PostInstallReadiness left nil: nothing was installed, so
// there is nothing new to recheck. If the recheck itself errors, the
// outcome's Kind still reflects the install's own success, but the error is
// returned and PostInstallReadiness is left nil -- a caller must treat that
// as "the recheck itself failed", not as "the install failed" or as a clean
// readiness result. This function does not interpret PostInstallReadiness; a
// caller decides whether the fresh result clears the gap that offered this
// setup.
func RunConfirmedSetupAndRecheckReadiness(ctx context.Context, preview SetupPreview, confirmed bool, dir, revision, configPath string) (SetupOutcome, error) {
	outcome, err := RunConfirmedSetup(ctx, preview, confirmed)
	if err != nil || outcome.Kind != SetupOutcomeSucceeded {
		return outcome, err
	}
	readiness, readinessErr := CheckProjectReadiness(dir, revision, configPath)
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
// mirrors project_ts_compiler_mise_probe.go's confinement pattern.
func setupExecutionEnv() []string {
	env := []string{"PATH=" + os.Getenv("PATH")}
	if home := os.Getenv("HOME"); home != "" {
		env = append(env, "HOME="+home)
	}
	return env
}
