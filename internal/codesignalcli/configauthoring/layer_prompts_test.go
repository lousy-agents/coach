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
			checkLayerStageNeverInfersPreselectsRecommends(t, tc)
		})
	}

	t.Run("a defined layer's prefixes are exactly what the user typed, never augmented with discovered roots", aDefinedLayersPrefixesAreExactlyWhatUser)

	t.Run("a blank prefix answer is never silently filled in with a discovered root", aBlankPrefixAnswerNeverSilentlyFilledDiscovered)
}

func aBlankPrefixAnswerNeverSilentlyFilledDiscovered(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api", "apps/web"}, Complete: true}

	result, _ := runAuthoring(discovered,
		"selected-root",
		"domain",
		"",
		"cancel",
	)

	expectNoPrefixAdoptsDiscoveredRoot(t, result, discovered)
	if len(result.Layers) != 0 {
		t.Fatalf("expected no layer to be recorded when its prefix answer was blank and then cancelled, got %+v", result.Layers)
	}
	if !result.Cancelled {
		t.Fatalf("expected Cancelled = true after cancelling the blank-prefix retry, got false")
	}
}

func aDefinedLayersPrefixesAreExactlyWhatUser(t *testing.T) {
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

// expectNoPrefixAdoptsDiscoveredRoot fails when any recorded layer prefix
// is one of the discovered roots: a prefix must be what the user typed.
func expectNoPrefixAdoptsDiscoveredRoot(t *testing.T, result Result, discovered projectmodel.TSRootDiscoveryResult) {
	t.Helper()
	for _, layer := range result.Layers {
		if root, adopted := adoptedDiscoveredRoot(layer.Prefixes, discovered.Roots); adopted {
			t.Fatalf("layer %q silently adopted discovered root %q as a prefix, Layers = %+v", layer.Name, root, result.Layers)
		}
	}
}
