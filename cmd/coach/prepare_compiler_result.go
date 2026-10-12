package main

import (
	"fmt"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

func reportPrepareCompilerMiseResult(result tssetup.PrepareCompilerMiseResult, stderr *os.File) int {
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
			fmt.Fprintf(stderr, "%s: mise install exited 0 but the installed TypeScript %s is not eligible (%s); expected the native platform package %s alongside it -- Coach does not attempt to repair or clean this up.\n", prepareCompilerMiseUsagePrefix, result.Version, result.Class, tstoolchain.NativeTypescriptPackageName())
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
