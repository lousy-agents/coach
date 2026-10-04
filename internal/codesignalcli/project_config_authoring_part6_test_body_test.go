package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_projectConfigAuthoringPart6Test_layersOnlyNoForbiddenPairsNoRequiredLayer_12(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
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

	wantLayers := []projectConfigLayer{
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

func body_projectConfigAuthoringPart6Test_layersAndForbiddenPairsNoRequiredLayer_42(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, _ := runAuthoring(discovered,
		"1",
		"domain", "internal/domain",
		"app", "internal/app",
		"",
		"domain", "app",
		"",
		"",
	)

	wantLayers := []projectConfigLayer{
		{Name: "domain", Prefixes: []string{"internal/domain"}},
		{Name: "app", Prefixes: []string{"internal/app"}},
	}
	if !equalLayers(result.Layers, wantLayers) {
		t.Fatalf("Layers = %+v, want %+v", result.Layers, wantLayers)
	}
	wantForbidden := []projectForbiddenImport{{From: "domain", To: "app"}}
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
