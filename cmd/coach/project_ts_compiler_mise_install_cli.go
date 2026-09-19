package main

import (
	"fmt"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli"
)

const prepareCompilerMiseUsagePrefix = "coach codesignal --baseline --prepare-compiler --project-language typescript"

// runPrepareCompilerMiseTypeScript dispatches `coach codesignal --baseline
// --prepare-compiler --project-language typescript`'s consented mise
// compiler-setup flow. The interactivity check on stdin runs before any
// revision resolution or readiness computation: with no controlling terminal
// -- or with one nobody is attending, which nonInteractiveRequested is what
// detects -- this never prompts and never mutates mise state.
func runPrepareCompilerMiseTypeScript(dir string, f codesignalFlags, stdin, stdout, stderr *os.File) int {
	if reason := interactiveRefusalReason(f, stdin); reason != "" {
		fmt.Fprintf(stderr, "%s: %s; refusing to enter interactive compiler setup or mutate mise state.\n", prepareCompilerMiseUsagePrefix, reason)
		return 2
	}
	return prepareCompilerMiseTypeScript(dir, stdin, stdout, stderr, f.projectConfig)
}

// prepareCompilerMiseTypeScript writes its entire interactive transcript
// (choice list, preview, and every prompt) to stderr, never stdout: this
// flow never produces a report of its own. stdout is accepted as a
// parameter only for signature symmetry with the rest of this package's CLI
// dispatch functions and is otherwise unused.
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

	ctx, stop := interruptibleContext()
	defer stop()
	result := codesignalcli.RunPrepareCompilerMiseSetup(ctx, dir, revision, projectConfigPath, readiness, stdin, stderr)
	return reportPrepareCompilerMiseResult(result, stderr)
}

func reportPrepareCompilerMiseResult(result codesignalcli.PrepareCompilerMiseResult, stderr *os.File) int {
	if result.PolicyRequired {
		fmt.Fprintf(stderr, "%s: a reviewed, committed policy is required before compiler setup; run guided policy authoring first (author_policy).\n", prepareCompilerMiseUsagePrefix)
		return 2
	}
	if result.RuntimeGapCode != "" {
		fmt.Fprintf(stderr, "%s: %s is a runtime-boundary gap; Coach has no compiler-setup command for it. Resolve the host Node runtime, then rerun --check-project.\n", prepareCompilerMiseUsagePrefix, result.RuntimeGapCode)
		return 2
	}
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
		case result.VerificationFailed():
			fmt.Fprintf(stderr, "%s: mise install exited 0 but the installed TypeScript %s is not eligible (%s); expected the native platform package %s alongside it -- Coach does not attempt to repair or clean this up.\n", prepareCompilerMiseUsagePrefix, result.Version, result.Class, codesignalcli.NativeTypescriptPackageName())
		case result.AttemptFailed():
			fmt.Fprintf(stderr, "%s: mise install failed; mise's install store may now contain a partial or failed install of TypeScript %s under the %s scope -- Coach does not attempt to clean this up.\n", prepareCompilerMiseUsagePrefix, result.Version, result.Choice)
		case result.NeverStarted():
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
