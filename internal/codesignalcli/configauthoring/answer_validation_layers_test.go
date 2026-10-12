package configauthoring

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func overlappingPrefixExplainsErrorRetryAcceptsCorrectedAnswer(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, out := runAuthoring(discovered,
		".",
		"domain", "internal/domain",
		"app", "internal/domain/sub",
		"retry",
		"internal/app",
		"",
		"",
		"",
	)

	if result.Cancelled {
		t.Fatalf("expected Cancelled = false after a successful retry, got true")
	}
	wantLayers := []projectconfig.Layer{
		{Name: "domain", Prefixes: []string{"internal/domain"}},
		{Name: "app", Prefixes: []string{"internal/app"}},
	}
	if !equalLayers(result.Layers, wantLayers) {
		t.Fatalf("Layers = %+v, want %+v", result.Layers, wantLayers)
	}
	if !strings.Contains(strings.ToLower(out), "overlap") && !strings.Contains(strings.ToLower(out), "invalid") {
		t.Fatalf("expected an explanatory error message about the overlapping prefix, got:\n%s", out)
	}
}

func forbiddenPairReferencingUndeclaredLayerExplainsErrorAllows(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, out := runAuthoring(discovered,
		".",
		"domain", "internal/domain",
		"",
		"unknown", "domain",
		"cancel",
	)

	if !result.Cancelled {
		t.Fatalf("expected Cancelled = true, got false")
	}
	if len(result.ForbiddenImports) != 0 {
		t.Fatalf("ForbiddenImports = %+v, want none", result.ForbiddenImports)
	}
	if !strings.Contains(out, "unknown") {
		t.Fatalf("expected the error explanation to reference the undeclared layer, got:\n%s", out)
	}
}
