package main

import (
	"context"
	"fmt"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli"
)

const prepareCompilerMiseUsagePrefix = "coach codesignal --baseline --prepare-compiler --project-language typescript"

// runPrepareCompilerMiseTypeScript dispatches `coach codesignal --baseline
// --prepare-compiler --project-language typescript`'s consented mise
// compiler-setup flow (coach#328 Task 5, AC-SET-1..AC-SET-8, AC-SET-19,
// AC-SET-22, AC-SET-23, AC-SET-24).
//
// Folding this flow into the normal-scan orchestration is coach#330's job;
// today it is a standalone `--prepare-compiler` invocation, never combined
// with a report-producing scan.
//
// The controlling-terminal check on stdin runs before any revision
// resolution or readiness computation: without a controlling terminal, this
// never prompts and never mutates mise state (AC-SET-24).
func runPrepareCompilerMiseTypeScript(dir string, stdin, stdout, stderr *os.File, projectConfigPath string) int {
	if !codesignalcli.HasControllingTerminal(stdin) {
		fmt.Fprintf(stderr, "%s: no controlling terminal is available; refusing to enter interactive compiler setup or mutate mise state.\n", prepareCompilerMiseUsagePrefix)
		return 2
	}
	return prepareCompilerMiseTypeScript(dir, stdin, stdout, stderr, projectConfigPath)
}

// prepareCompilerMiseTypeScript performs runPrepareCompilerMiseTypeScript's
// work after the controlling-terminal gate: resolve the baseline revision,
// compute readiness, run the interactive session, and translate its outcome
// into an exit code.
//
// The interactive transcript (choice list, the AC-SET-2 preview, and every
// prompt) is written to stderr, never stdout: this flow never produces a
// report of its own (that is a later scan step's job), so stdout stays
// empty on every outcome -- cancelled, failed, or successful alike -- the
// same way a cancelled/declined guided-authoring session leaves stdout
// empty. stdout is accepted as a parameter only for signature symmetry with
// the rest of this package's CLI dispatch functions and is otherwise
// unused.
func prepareCompilerMiseTypeScript(dir string, stdin, stdout, stderr *os.File, projectConfigPath string) int {
	revision, err := codesignalcli.ResolveBaselineRevision(dir)
	if err != nil {
		fmt.Fprintf(stderr, "%s: could not resolve the baseline revision: %s\n", prepareCompilerMiseUsagePrefix, err)
		return 3
	}

	readiness, err := codesignalcli.CheckProjectReadiness(dir, revision, projectConfigPath)
	if err != nil {
		fmt.Fprintf(stderr, "%s: could not compute project readiness: %s\n", prepareCompilerMiseUsagePrefix, err)
		return 3
	}

	result := codesignalcli.RunPrepareCompilerMiseSetup(context.Background(), dir, revision, projectConfigPath, readiness, stdin, stderr)
	return reportPrepareCompilerMiseResult(result, stderr)
}

// reportPrepareCompilerMiseResult maps one PrepareCompilerMiseResult to this
// dispatch's exit code. Exit 2 covers both AC-SET-7 (setup failure) and
// AC-SET-8 (cancellation): neither ever emits a report, and neither ever
// attempts a rollback of a partially-applied install. NoChoicesOffered is
// deliberately distinct from both -- nothing was attempted and nothing was
// declined, so it exits 0 with an explanatory message rather than being
// reported as though setup itself had failed.
//
// The !Succeeded branch further distinguishes three causes so the user is
// never told an install failed when the subprocess itself succeeded
// (coach#328 Task 5 integration repair, Finding 2): a verification failure
// (the subprocess exited 0, but the installed package did not classify as
// eligible), an observed subprocess failure (a just-run install that timed
// out, overflowed its output budget, or exited non-zero), and a install that
// could never even be started (mise absent from PATH, or -- distinguished by
// a non-empty Code, Finding 5 -- the install's own insulation could not be
// established).
func reportPrepareCompilerMiseResult(result codesignalcli.PrepareCompilerMiseResult, stderr *os.File) int {
	if result.NoChoicesOffered {
		fmt.Fprintf(stderr, "%s: no executable mise compiler-setup choice is currently offered; nothing to set up.\n", prepareCompilerMiseUsagePrefix)
		return 0
	}
	if result.Cancelled {
		fmt.Fprintf(stderr, "%s: setup was cancelled or not confirmed; no report was produced and no mise state was changed.\n", prepareCompilerMiseUsagePrefix)
		return 2
	}
	if !result.Trusted {
		fmt.Fprintf(stderr, "%s: the selected mise scope refused (%s); no report was produced.\n", prepareCompilerMiseUsagePrefix, result.Code)
		return 2
	}
	if !result.Succeeded {
		switch {
		case result.Observed && result.Class != "":
			fmt.Fprintf(stderr, "%s: mise install exited 0 but the installed TypeScript %s is not eligible (%s); expected the native platform package %s alongside it -- Coach does not attempt to repair or clean this up.\n", prepareCompilerMiseUsagePrefix, result.Version, result.Class, codesignalcli.NativeTypescriptPackageName())
		case result.Attempted:
			fmt.Fprintf(stderr, "%s: mise install failed; mise's install store may now contain a partial or failed install of TypeScript %s under the %s scope -- Coach does not attempt to clean this up.\n", prepareCompilerMiseUsagePrefix, result.Version, result.Choice)
		case result.Code != "":
			fmt.Fprintf(stderr, "%s: mise install could not even be started (%s).\n", prepareCompilerMiseUsagePrefix, result.Code)
		default:
			fmt.Fprintf(stderr, "%s: mise install could not even be started.\n", prepareCompilerMiseUsagePrefix)
		}
		return 2
	}

	state, version := "", ""
	if result.PostInstallReadiness != nil {
		state = string(result.PostInstallReadiness.Checks.Compiler.State)
		version = result.PostInstallReadiness.Checks.Compiler.Version
	}
	fmt.Fprintf(stderr, "%s: installed TypeScript %s via %s; rerun readiness reports compiler check %s (version=%s).\n", prepareCompilerMiseUsagePrefix, result.Version, result.Origin, state, version)
	return 0
}
