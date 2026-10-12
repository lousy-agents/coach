package main

import (
	"fmt"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"
)

// runScanCompilerSetupOffer implements the interactive compiler-setup offer
// for a real scan's CompilerUnresolvedError gap (AC-SET-9): it consumes the
// readiness snapshot already computed alongside the gap (wrapped.Readiness)
// rather than recomputing it a third time, runs the combined setup menu,
// and -- only when the confirmed install succeeds and its AC-SET-6
// readiness rerun reports ready or ready_with_limits --
// retries the same scan once more (compiler-setup consumed from the offer
// budget: AC-SET-9's offer runs at most once per invocation), so the customer's original
// invocation still produces a report. Every other outcome (no choices
// offered, cancelled, failed, or a rerun that still reports a gap) exits 2
// with no CodeSignal report ever rendered (AC-18); wrapped.RemediationLine()
// is printed for every one of those outcomes, but withheld on the
// succeeded-and-continued path, where the gap it names no longer exists.
// The gap the offer exists for is named before the menu opens rather than
// here -- RunCompilerSetupOffer is where it is known that a menu will
// actually open, and diagnosing it here would also promise choices to a
// customer about to be told there are none.
func runScanCompilerSetupOffer(dir string, f codesignalFlags, stdout, stderr *os.File, wrapped *tssetup.CompilerUnresolvedErrorWithReadiness, budget scanOfferBudget) int {
	ctx, stop := interruptibleContext()
	defer stop()
	result := tssetup.RunCompilerSetupOffer(ctx, dir, wrapped.Revision, wrapped.ConfigPath, wrapped.Code, wrapped.Readiness, os.Stdin, stderr)

	if shouldContinueAfterSetup(result) {
		return runCodesignalScan(dir, f, stdout, stderr, budget.without(scanOfferCompilerSetup))
	}

	if result.PolicyRequired {
		fmt.Fprintln(stderr, wrapped.RemediationLine())
		fmt.Fprintln(stderr, "coach codesignal: a reviewed, committed policy is required before compiler setup; run guided policy authoring first (author_policy).")
		return 2
	}

	if result.RuntimeGapCode != "" {
		fmt.Fprintln(stderr, wrapped.RemediationLine())
		fmt.Fprintf(stderr, "coach codesignal: %s is a runtime-boundary gap; Coach has no compiler-setup command for it.\n", result.RuntimeGapCode)
		return 2
	}

	fmt.Fprintln(stderr, wrapped.RemediationLine())

	if result.NoChoicesOffered {
		if line := withheldSetupChoicesLine(result.Withheld); line != "" {
			fmt.Fprintln(stderr, line)
		}
		return 2
	}
	if result.Cancelled {
		fmt.Fprintln(stderr, "coach codesignal: compiler setup was cancelled or not confirmed; no report was produced and no setup command ran.")
		return 2
	}
	if !result.Succeeded {
		fmt.Fprintf(stderr, "coach codesignal: compiler setup failed (%s); no report was produced.\n", result.FailureDetail)
		if line := setupResidueDisclosure(result); line != "" {
			fmt.Fprintln(stderr, line)
		}
		return 2
	}
	if result.PostInstallReadiness == nil {
		fmt.Fprintln(stderr, "coach codesignal: compiler setup succeeded, but the readiness recheck itself failed to run; no report was produced.")
		return 2
	}
	fmt.Fprintln(stderr, "coach codesignal: compiler setup succeeded, but the readiness recheck still reports a gap; no report was produced.")
	return 2
}

// shouldContinueAfterSetup is AC-SET-6/AC-7's continuation gate: the same
// scan may resume after a consented compiler-setup action only when that
// action actually completed (Succeeded) and its own mandatory readiness
// rerun (PostInstallReadiness) reports no gap (readinessAllowsScan). A
// successful install whose rerun still reports needs_prerequisite/
// needs_policy/outside_support must still exit 2, not resume: the install
// ran, but it did not resolve what the scan needed.
func shouldContinueAfterSetup(result tssetup.CompilerSetupOfferResult) bool {
	return result.Succeeded && result.PostInstallReadiness != nil && readinessAllowsScan(result.PostInstallReadiness.Status)
}

func readinessAllowsScan(status projectreadiness.Status) bool {
	return status == projectreadiness.StatusReady || status == projectreadiness.StatusReadyWithLimits
}
