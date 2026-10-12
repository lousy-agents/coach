package main

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"
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
		expectFailedSetupNeverContinues(t, allStatuses)
	})

	t.Run("Succeeded true with a nil PostInstallReadiness never continues", func(t *testing.T) {
		result := tssetup.CompilerSetupOfferResult{Succeeded: true, PostInstallReadiness: nil}
		if shouldContinueAfterSetup(result) {
			t.Errorf("shouldContinueAfterSetup(Succeeded=true, PostInstallReadiness=nil) = true, want false: a rerun that never produced a readiness result must never let the scan continue")
		}
	})

	for _, status := range allStatuses {
		status := status
		want := status == projectreadiness.StatusReady || status == projectreadiness.StatusReadyWithLimits
		t.Run("Succeeded true with PostInstallReadiness.Status="+string(status), func(t *testing.T) {
			expectSucceededSetupContinuation(t, status, want)
		})
	}
}

func expectFailedSetupNeverContinues(t *testing.T, statuses []projectreadiness.Status) {
	t.Helper()
	for _, status := range statuses {
		readiness := projectreadiness.Result{Status: status}
		result := tssetup.CompilerSetupOfferResult{Succeeded: false, PostInstallReadiness: &readiness}
		if shouldContinueAfterSetup(result) {
			t.Errorf("shouldContinueAfterSetup(Succeeded=false, PostInstallReadiness.Status=%s) = true, want false: an install that did not succeed must never let the scan continue", status)
		}
	}
}

func expectSucceededSetupContinuation(t *testing.T, status projectreadiness.Status, want bool) {
	t.Helper()
	readiness := projectreadiness.Result{Status: status}
	result := tssetup.CompilerSetupOfferResult{Succeeded: true, PostInstallReadiness: &readiness}
	if got := shouldContinueAfterSetup(result); got != want {
		t.Errorf("shouldContinueAfterSetup(Succeeded=true, PostInstallReadiness.Status=%s) = %v, want %v", status, got, want)
	}
}
