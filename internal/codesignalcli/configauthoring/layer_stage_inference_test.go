package configauthoring

import (
	"slices"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func checkLayerStageNeverInfersPreselectsRecommends(t *testing.T, tc struct {
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

func adoptedDiscoveredRoot(prefixes, roots []string) (string, bool) {
	for _, prefix := range prefixes {
		if slices.Contains(roots, prefix) {
			return prefix, true
		}
	}
	return "", false
}
