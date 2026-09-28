package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_projectConfigAuthoringPart5Test_aBlankPrefixAnswerIsNeverSilentlyFilledInWithADi_86(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{Roots: []string{"apps/api", "apps/web"}, Complete: true}

	result, _ := runAuthoring(discovered,
		"selected-root",
		"domain",
		"",
		"cancel",
	)

	for _, layer := range result.Layers {
		for _, prefix := range layer.Prefixes {
			for _, root := range discovered.Roots {
				if prefix == root {
					t.Fatalf("layer %q silently adopted discovered root %q as a prefix, Layers = %+v", layer.Name, root, result.Layers)
				}
			}
		}
	}
	if len(result.Layers) != 0 {
		t.Fatalf("expected no layer to be recorded when its prefix answer was blank and then cancelled, got %+v", result.Layers)
	}
	if !result.Cancelled {
		t.Fatalf("expected Cancelled = true after cancelling the blank-prefix retry, got false")
	}
}
