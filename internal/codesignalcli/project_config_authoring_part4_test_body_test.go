package codesignalcli

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_projectConfigAuthoringPart4Test_anEmptySelectionIsRejectedAndCancelStopsTheSessi_16(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, out := runAuthoring(discovered,
		"",
		"cancel",
	)
	if !result.Cancelled {
		t.Fatalf("expected an empty root selection to be rejected and the cancel answer to stop the session, got Cancelled = false, result = %+v", result)
	}
	if len(result.Roots) != 0 {
		t.Fatalf("expected no roots to be recorded for a cancelled empty selection, got %v", result.Roots)
	}
	if !strings.Contains(out, "at least one") {
		t.Fatalf("expected an explanation that at least one root is required, got:\n%s", out)
	}
	if strings.Contains(strings.ToLower(out), "layer") {
		t.Fatalf("expected authoring to stop at cancellation, not continue to the layer stage, got:\n%s", out)
	}
}

func body_projectConfigAuthoringPart4Test_anEmptySelectionIsRejectedAndRetryAcceptsACorrec_35(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, _ := runAuthoring(discovered,
		"",
		"retry",
		"1",
		"",
		"",
		"",
	)
	if result.Cancelled {
		t.Fatalf("expected Cancelled = false after a successful retry, got true")
	}
	if !equalStringSlices(result.Roots, []string{"apps/api"}) {
		t.Fatalf("Roots = %v, want [apps/api]", result.Roots)
	}
}

func body_projectConfigAuthoringPart4Test_anAbsolutePathIsRejectedAndCancelStopsTheSession_52(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, out := runAuthoring(discovered,
		"/abs/path",
		"cancel",
	)
	if !result.Cancelled {
		t.Fatalf("expected an absolute root path to be rejected and cancelled, got Cancelled = false")
	}
	if !strings.Contains(out, "/abs/path") {
		t.Fatalf("expected the error explanation to reference the offending path, got:\n%s", out)
	}
}

func body_projectConfigAuthoringPart4Test_aPathEscapingTheRepositoryIsRejectedAndRetryAcce_65(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, _ := runAuthoring(discovered,
		"../x",
		"retry",
		"services/checkout",
		"",
		"",
		"",
	)
	if result.Cancelled {
		t.Fatalf("expected Cancelled = false after a successful retry, got true")
	}
	if !equalStringSlices(result.Roots, []string{"services/checkout"}) {
		t.Fatalf("Roots = %v, want [services/checkout]", result.Roots)
	}
}
