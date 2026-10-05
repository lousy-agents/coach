package main

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// TestShouldContinueAfterSetup pins AC-SET-6/AC-7's continuation gate: the
// same scan may resume after a consented compiler-setup action only when
// the action itself succeeded AND its own mandatory readiness rerun reports
// no gap. Every ReadinessStatus value is exercised, so a status this table
// omits cannot silently regress to the wrong side of the gate.
func TestShouldContinueAfterSetup(t *testing.T) {
	allStatuses := []projectreadiness.Status{
		projectreadiness.StatusOutsideSupport,
		projectreadiness.StatusNeedsPrerequisite,
		projectreadiness.StatusNeedsPolicy,
		projectreadiness.StatusReadyWithLimits,
		projectreadiness.StatusReady,
	}

	t.Run("Succeeded false never continues, regardless of PostInstallReadiness", func(t *testing.T) {
		body_mainPart2Test_SucceededFalseNeverContinuesRegardlessOfPostInst_23(t, allStatuses)
	})

	t.Run("Succeeded true with a nil PostInstallReadiness never continues", func(t *testing.T) {
		body_mainPart2Test_SucceededTrueWithANilPostInstallReadinessNeverCo_34(t)
	})

	for _, status := range allStatuses {
		status := status
		want := status == projectreadiness.StatusReady || status == projectreadiness.StatusReadyWithLimits
		t.Run("Succeeded true with PostInstallReadiness.Status="+string(status), func(t *testing.T) {
			body_mainPart2Test_44(t, status, want)
		})
	}
}

func TestAnalysisErrorReportForRuntimeUnresolvedError(t *testing.T) {
	err := &codesignalcli.RuntimeUnresolvedError{Code: projectreadiness.GapNodeMissing, ConfigPath: "project.json"}
	got := analysisErrorReportFor(err, "typescript", false)
	if got.exitCode != 2 {
		t.Fatalf("analysisErrorReportFor(RuntimeUnresolvedError).exitCode = %d, want 2", got.exitCode)
	}
	if len(got.lines) != 1 {
		t.Fatalf("analysisErrorReportFor(RuntimeUnresolvedError).lines = %q, want exactly the gap line", got.lines)
	}
	if got.lines[0] != err.RemediationLine() {
		t.Fatalf("analysisErrorReportFor(RuntimeUnresolvedError).lines[0] = %q, want %q", got.lines[0], err.RemediationLine())
	}
}

// TestWithheldSetupChoicesLine pins the disclosure that breaks the
// --check-project loop when nothing at all could be offered, and its silence
// on the runtime-boundary path where no menu was ever built.
func TestWithheldSetupChoicesLine(t *testing.T) {
	t.Run("lists every withheld kind and reason when nothing is executable", func(t *testing.T) {
		body_mainPart2Test_listsEveryWithheldKindAndReasonWhenNothingIsExec_72(t)
	})
	t.Run("is silent when no menu was built", func(t *testing.T) {
		body_mainPart2Test_isSilentWhenNoMenuWasBuilt_82(t)
	})
}

func TestScanShouldOfferCompilerSetupIgnoresRuntimeUnresolvedError(t *testing.T) {
	err := &codesignalcli.RuntimeUnresolvedError{Code: projectreadiness.GapNodeMissing, ConfigPath: "project.json"}
	if wrapped, ok := scanShouldOfferCompilerSetup(err, false); ok {
		t.Fatalf("scanShouldOfferCompilerSetup(RuntimeUnresolvedError) = (%v, true), want false: a runtime gap must not enter the compiler-setup offer", wrapped)
	}
}
