package configauthoring

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func layersForbiddenPairsRequiredLayer(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, _ := runAuthoring(discovered,
		"1",
		"domain", "internal/domain",
		"app", "internal/app",
		"",
		"domain", "app",
		"",
		"domain",
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
	if result.RequiredLayer != "domain" {
		t.Fatalf("RequiredLayer = %q, want %q", result.RequiredLayer, "domain")
	}
	if result.Cancelled {
		t.Fatalf("Cancelled = true, want false")
	}
}
