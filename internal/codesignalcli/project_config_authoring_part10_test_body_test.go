package codesignalcli

import (
	"strings"
	"testing"
	"time"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_projectConfigAuthoringPart10Test_decliningIsNotACancellation_36(t *testing.T, tc struct {
	name   string
	answer string
}, result AuthoringResult) {
	if result.Cancelled {
		t.Fatalf("expected Cancelled = false for declined answer %q (a later write stage must gate on Approved, not !Cancelled), got true", tc.answer)
	}
}

func body_projectConfigAuthoringPart10Test_approvesWithTheExactApprovalToken_44(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, _ := runAuthoring(discovered,
		"1",
		"api", "apps/api",
		"",
		"",
		"",
		"approve",
	)
	if !result.Approved {
		t.Fatalf("expected Approved = true for the exact approval token, got false")
	}
}

func body_projectConfigAuthoringPart10Test_approvesCaseInsensitively_58(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, _ := runAuthoring(discovered,
		"1",
		"api", "apps/api",
		"",
		"",
		"",
		"APPROVE",
	)
	if !result.Approved {
		t.Fatalf("expected Approved = true for a case-insensitive approval token, got false")
	}
}

func body_projectConfigAuthoringPart10Test_134(t *testing.T, discovered projectmodel.TSRootDiscoveryResult, watchdog time.Duration, tc struct {
	name       string
	lines      []string
	wantSubstr string
	wantLayers []projectConfigLayer
}) {
	result, out := runAuthoringWithTimeout(t, watchdog, discovered, tc.lines...)
	if !result.Cancelled {
		t.Fatalf("expected exhausted input at a retry prompt to cancel the session, got Cancelled = false, result = %+v", result)
	}
	if !strings.Contains(out, tc.wantSubstr) {
		t.Fatalf("expected the transcript to reach its named stage (rejection text %q), got:\n%s", tc.wantSubstr, out)
	}
	if !equalLayers(result.Layers, tc.wantLayers) {
		t.Fatalf("Layers = %+v, want %+v (the case's own stage must reject with the declared layer already accepted, not with zero layers)", result.Layers, tc.wantLayers)
	}
}
