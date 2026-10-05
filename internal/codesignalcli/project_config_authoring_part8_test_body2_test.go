package codesignalcli

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_projectConfigAuthoringPart8Test_requiredLayerNamingAnUndeclaredLayerExplainsTheE_83(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, out := runAuthoring(discovered,
		".",
		"domain", "internal/domain",
		"",
		"",
		"unknown",
		"retry",
		"domain",
	)

	if result.Cancelled {
		t.Fatalf("expected Cancelled = false after a successful retry, got true")
	}
	if result.RequiredLayer != "domain" {
		t.Fatalf("RequiredLayer = %q, want %q", result.RequiredLayer, "domain")
	}
	if !strings.Contains(out, "unknown") {
		t.Fatalf("expected the error explanation to reference the undeclared layer, got:\n%s", out)
	}
}
