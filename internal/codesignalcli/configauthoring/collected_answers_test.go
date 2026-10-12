package configauthoring

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func TestAuthorProjectConfig_CollectsLayersForbiddenPairsAndRequiredLayerInOrder(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api"}, Complete: true}

	t.Run("layers only: no forbidden pairs, no required layer", func(t *testing.T) {
		layersOnlyNoForbiddenPairsNoRequiredLayer(t, discovered)
	})

	t.Run("layers and forbidden pairs, no required layer", func(t *testing.T) {
		layersForbiddenPairsNoRequiredLayer(t, discovered)
	})

	t.Run("layers, forbidden pairs, and a required layer", func(t *testing.T) {
		layersForbiddenPairsRequiredLayer(t, discovered)
	})

	t.Run("keeps the accepted root and layer after later stages are left blank", func(t *testing.T) {
		keepsAcceptedRootLayerAfterLaterStagesAre(t, discovered)
	})
}

func layersOnlyNoForbiddenPairsNoRequiredLayer(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, _ := runAuthoring(discovered,
		"1",
		"domain",
		"internal/domain",
		"app",
		"internal/app",
		"",
		"",
		"",
	)

	wantLayers := []projectconfig.Layer{
		{Name: "domain", Prefixes: []string{"internal/domain"}},
		{Name: "app", Prefixes: []string{"internal/app"}},
	}
	if !equalLayers(result.Layers, wantLayers) {
		t.Fatalf("Layers = %+v, want %+v", result.Layers, wantLayers)
	}
	if len(result.ForbiddenImports) != 0 {
		t.Fatalf("ForbiddenImports = %+v, want none", result.ForbiddenImports)
	}
	if result.RequiredLayer != "" {
		t.Fatalf("RequiredLayer = %q, want empty", result.RequiredLayer)
	}
	if result.Cancelled {
		t.Fatalf("Cancelled = true, want false")
	}
}

func layersForbiddenPairsNoRequiredLayer(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, _ := runAuthoring(discovered,
		"1",
		"domain", "internal/domain",
		"app", "internal/app",
		"",
		"domain", "app",
		"",
		"",
	)

	wantLayers := []projectconfig.Layer{
		{Name: "domain", Prefixes: []string{"internal/domain"}},
		{Name: "app", Prefixes: []string{"internal/app"}},
	}
	if !equalLayers(result.Layers, wantLayers) {
		t.Fatalf("Layers = %+v, want %+v", result.Layers, wantLayers)
	}
	wantForbidden := []projectconfig.ForbiddenImport{{From: "domain", To: "app"}}
	if !equalForbiddenImports(result.ForbiddenImports, wantForbidden) {
		t.Fatalf("ForbiddenImports = %+v, want %+v", result.ForbiddenImports, wantForbidden)
	}
	if result.RequiredLayer != "" {
		t.Fatalf("RequiredLayer = %q, want empty", result.RequiredLayer)
	}
	if result.Cancelled {
		t.Fatalf("Cancelled = true, want false")
	}
}

func keepsAcceptedRootLayerAfterLaterStagesAre(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, _ := runAuthoring(discovered,
		"1",
		"domain", "internal/domain",
		"",
		"",
		"",
	)

	if !equalStringSlices(result.Roots, []string{"apps/api"}) {
		t.Fatalf("Roots = %v, want [apps/api]", result.Roots)
	}
	wantLayers := []projectconfig.Layer{{Name: "domain", Prefixes: []string{"internal/domain"}}}
	if !equalLayers(result.Layers, wantLayers) {
		t.Fatalf("Layers = %+v, want %+v", result.Layers, wantLayers)
	}
}
