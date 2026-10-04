package main

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli"
)

func body_mainPart2Test_SucceededFalseNeverContinuesRegardlessOfPostInst_23(t *testing.T, allStatuses []codesignalcli.ReadinessStatus) {
	for _, status := range allStatuses {
		status := status
		readiness := codesignalcli.ReadinessResult{Status: status}
		result := codesignalcli.CompilerSetupOfferResult{Succeeded: false, PostInstallReadiness: &readiness}
		if shouldContinueAfterSetup(result) {
			t.Errorf("shouldContinueAfterSetup(Succeeded=false, PostInstallReadiness.Status=%s) = true, want false: an install that did not succeed must never let the scan continue", status)
		}
	}
}

func body_mainPart2Test_SucceededTrueWithANilPostInstallReadinessNeverCo_34(t *testing.T) {
	result := codesignalcli.CompilerSetupOfferResult{Succeeded: true, PostInstallReadiness: nil}
	if shouldContinueAfterSetup(result) {
		t.Errorf("shouldContinueAfterSetup(Succeeded=true, PostInstallReadiness=nil) = true, want false: a rerun that never produced a readiness result must never let the scan continue")
	}
}

func body_mainPart2Test_44(t *testing.T, status codesignalcli.ReadinessStatus, want bool) {
	readiness := codesignalcli.ReadinessResult{Status: status}
	result := codesignalcli.CompilerSetupOfferResult{Succeeded: true, PostInstallReadiness: &readiness}
	if got := shouldContinueAfterSetup(result); got != want {
		t.Errorf("shouldContinueAfterSetup(Succeeded=true, PostInstallReadiness.Status=%s) = %v, want %v", status, got, want)
	}
}

func body_mainPart2Test_listsEveryWithheldKindAndReasonWhenNothingIsExec_72(t *testing.T) {
	line := withheldSetupChoicesLine([]codesignalcli.WithheldSetupChoice{
		{Kind: codesignalcli.SetupChoiceProjectPackage, Reason: "manifest_declaration"},
		{Kind: codesignalcli.SetupChoiceProjectMise, Reason: "mise_unconfigured"},
	})
	want := "coach codesignal: no compiler-setup choice is executable here: project_package (manifest_declaration), project_mise (mise_unconfigured)."
	if line != want {
		t.Fatalf("withheldSetupChoicesLine() = %q, want %q", line, want)
	}
}

func body_mainPart2Test_isSilentWhenNoMenuWasBuilt_82(t *testing.T) {
	if got := withheldSetupChoicesLine(nil); got != "" {
		t.Fatalf("withheldSetupChoicesLine(nil) = %q, want empty: no menu was built, so there is nothing to explain", got)
	}
}
