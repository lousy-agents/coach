package configauthoring

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func TestAuthorProjectConfig_InvalidAnswersExplainAndAllowRetryOrCancel(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Complete: true}

	t.Run("duplicate layer name explains the error and cancel stops the session there", func(t *testing.T) {
		duplicateLayerNameExplainsErrorCancelStopsSession(t, discovered)
	})

	t.Run("overlapping prefix explains the error and retry accepts a corrected answer", func(t *testing.T) {
		overlappingPrefixExplainsErrorRetryAcceptsCorrectedAnswer(t, discovered)
	})

	t.Run("forbidden pair referencing an undeclared layer explains the error and allows cancel", func(t *testing.T) {
		forbiddenPairReferencingUndeclaredLayerExplainsErrorAllows(t, discovered)
	})

	t.Run("required layer naming an undeclared layer explains the error and retry accepts a declared layer", func(t *testing.T) {
		requiredLayerNamingUndeclaredLayerExplainsErrorRetry(t, discovered)
	})
}

func requiredLayerNamingUndeclaredLayerExplainsErrorRetry(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
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

func duplicateLayerNameExplainsErrorCancelStopsSession(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, out := runAuthoring(discovered,
		".",
		"domain", "internal/domain",
		"domain",
		"cancel",
	)

	if !result.Cancelled {
		t.Fatalf("expected Cancelled = true, got false")
	}
	wantLayers := []projectconfig.Layer{{Name: "domain", Prefixes: []string{"internal/domain"}}}
	if !equalLayers(result.Layers, wantLayers) {
		t.Fatalf("Layers = %+v, want only the first accepted layer %+v", result.Layers, wantLayers)
	}
	if !strings.Contains(out, "domain") {
		t.Fatalf("expected the error explanation to reference the offending answer, got:\n%s", out)
	}
	if strings.Contains(strings.ToLower(out), "forbidden import") {
		t.Fatalf("expected authoring to stop at cancellation, not continue to the forbidden-import stage, got:\n%s", out)
	}
}

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
