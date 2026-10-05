package configauthoring

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func TestAuthorProjectConfig_LayerPrefixesRejectOversizedBudget(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Complete: true}

	prefixes := make([]string, projectconfig.MaxLayerPrefixes+1)
	for i := range prefixes {
		prefixes[i] = fmt.Sprintf("dir%d", i)
	}

	result, out := runAuthoringWithTimeout(t, 3*time.Second, discovered,
		".",
		"domain",
		strings.Join(prefixes, ","),
		"cancel",
	)

	if !result.Cancelled {
		t.Fatalf("expected Cancelled = true after an oversized prefix list is rejected and cancelled, got false")
	}
	if len(result.Layers) != 0 {
		t.Fatalf("expected no layer to be recorded for a rejected oversized prefix list, got %+v", result.Layers)
	}
	if !strings.Contains(out, fmt.Sprintf("%d", projectconfig.MaxLayerPrefixes)) {
		t.Fatalf("expected the rejection explanation to reference the %d-entry budget, got:\n%s", projectconfig.MaxLayerPrefixes, out)
	}
}

func TestAuthorProjectConfig_LayerStageNeverInfersPreselectsOrRecommends(t *testing.T) {
	cases := []struct {
		name       string
		discovered projectmodel.TSRootDiscoveryResult
	}{
		{
			name:       "blank answers preselect nothing when discovered roots are present",
			discovered: projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api", "apps/web"}, Complete: true},
		},
		{
			name:       "blank answers preselect nothing when roots and candidates are present",
			discovered: projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api"}, Candidates: []string{"libs/shared"}, Complete: true},
		},
		{
			name:       "blank answers preselect nothing when nothing is discovered",
			discovered: projectmodel.TSRootDiscoveryResult{Complete: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body_projectConfigAuthoringPart5Test_30(t, tc)
		})
	}

	t.Run("a defined layer's prefixes are exactly what the user typed, never augmented with discovered roots", func(t *testing.T) {
		body_projectConfigAuthoringPart5Test_aDefinedLayerSPrefixesAreExactlyWhatTheUserTyped_69(t)
	})

	t.Run("a blank prefix answer is never silently filled in with a discovered root", func(t *testing.T) {
		body_projectConfigAuthoringPart5Test_aBlankPrefixAnswerIsNeverSilentlyFilledInWithADi_86(t)
	})
}

func body_projectConfigAuthoringPart5Test_aBlankPrefixAnswerIsNeverSilentlyFilledInWithADi_86(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api", "apps/web"}, Complete: true}

	result, _ := runAuthoring(discovered,
		"selected-root",
		"domain",
		"",
		"cancel",
	)

	for _, layer := range result.Layers {
		(&sigbodyprojectConfigAuthoringPart5TestaBlankPrefixAnswerIsNe{discovered: discovered, layer: layer, result: result, t: t}).call()

	}
	if len(result.Layers) != 0 {
		t.Fatalf("expected no layer to be recorded when its prefix answer was blank and then cancelled, got %+v", result.Layers)
	}
	if !result.Cancelled {
		t.Fatalf("expected Cancelled = true after cancelling the blank-prefix retry, got false")
	}
}

func body_projectConfigAuthoringPart5Test_30(t *testing.T, tc struct {
	name       string
	discovered projectmodel.TSRootDiscoveryResult
}) {

	result, out := runAuthoring(tc.discovered,
		"selected-root",
		"",
		"",
		"",
	)

	if len(result.Layers) != 0 {
		t.Fatalf("expected no layers to be preselected when the user answered blank, got %+v", result.Layers)
	}
	if len(result.ForbiddenImports) != 0 {
		t.Fatalf("expected no forbidden pairs to be preselected, got %+v", result.ForbiddenImports)
	}
	if result.RequiredLayer != "" {
		t.Fatalf("expected no required layer to be preselected, got %q", result.RequiredLayer)
	}

	printed := strings.ToLower(out)
	forbiddenPhrases := []string{"suggested layer", "we suggest", "based on your roots", "recommended layer"}
	for _, phrase := range forbiddenPhrases {
		if strings.Contains(printed, phrase) {
			t.Fatalf("layer-collection prompts must never recommend a layer (found %q), got output:\n%s", phrase, out)
		}
	}

	beforeCoveragePreview := out
	if idx := strings.Index(out, "Coverage preview:"); idx != -1 {
		beforeCoveragePreview = out[:idx]
	}
	for _, root := range tc.discovered.Roots {
		if strings.Count(beforeCoveragePreview, root) > 1 {
			t.Fatalf("expected discovered root %q to appear only once before the coverage preview (the root-selection suggestion), got it repeated in:\n%s", root, beforeCoveragePreview)
		}
	}
}

func body_projectConfigAuthoringPart5Test_aDefinedLayerSPrefixesAreExactlyWhatTheUserTyped_69(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api", "apps/web"}, Complete: true}

	result, _ := runAuthoring(discovered,
		"selected-root",
		"domain", "internal/domain",
		"",
		"",
		"",
	)

	wantLayers := []projectconfig.Layer{{Name: "domain", Prefixes: []string{"internal/domain"}}}
	if !equalLayers(result.Layers, wantLayers) {
		t.Fatalf("Layers = %+v, want exactly %+v with no discovered roots appended", result.Layers, wantLayers)
	}
}

func TestAuthorProjectConfig_CollectsLayersForbiddenPairsAndRequiredLayerInOrder(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api"}, Complete: true}

	t.Run("layers only: no forbidden pairs, no required layer", func(t *testing.T) {
		body_projectConfigAuthoringPart6Test_layersOnlyNoForbiddenPairsNoRequiredLayer_12(t, discovered)
	})

	t.Run("layers and forbidden pairs, no required layer", func(t *testing.T) {
		body_projectConfigAuthoringPart6Test_layersAndForbiddenPairsNoRequiredLayer_42(t, discovered)
	})

	t.Run("layers, forbidden pairs, and a required layer", func(t *testing.T) {
		body_projectConfigAuthoringPart6Test_layersForbiddenPairsAndARequiredLayer_72(t, discovered)
	})

	t.Run("keeps the accepted root and layer after later stages are left blank", func(t *testing.T) {
		body_projectConfigAuthoringPart6Test_keepsTheAcceptedRootAndLayerAfterLaterStagesAreL_102(t, discovered)
	})
}

func body_projectConfigAuthoringPart6Test_layersForbiddenPairsAndARequiredLayer_72(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
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

func body_projectConfigAuthoringPart6Test_keepsTheAcceptedRootAndLayerAfterLaterStagesAreL_102(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
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

type sigcallS044425946 struct {
	prefix    string
	v44425814 *sigbodyprojectConfigAuthoringPart5TestaBlankPrefixAnswerIsNe
}

func (sigRecv *sigcallS044425946) call() {

	for _, root := range sigRecv.v44425814.discovered.Roots {
		if sigRecv.prefix == root {
			sigRecv.v44425814.
				t.
				Fatalf("layer %q silently adopted discovered root %q as a prefix, Layers = %+v", sigRecv.v44425814.layer.Name, root, sigRecv.v44425814.result.Layers)
		}
	}
}

type sigbodyprojectConfigAuthoringPart5TestaBlankPrefixAnswerIsNe struct {
	discovered projectmodel.
			TSRootDiscoveryResult
	layer  projectconfig.Layer
	result Result
	t      *testing.
		T
}

func (sigRecv *sigbodyprojectConfigAuthoringPart5TestaBlankPrefixAnswerIsNe) call() {

	for _, prefix := range sigRecv.layer.Prefixes {
		(&sigcallS044425946{prefix: prefix, v44425814: sigRecv}).call()

	}
}
