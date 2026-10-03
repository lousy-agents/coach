package codesignalcli

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_projectConfigAuthoringPart2Test_mixedLayerMatches_17(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, out := runAuthoring(discovered,
		"1,2",
		"libsLayer", "libs",
		"apiLayer", "apps/api",
		"unusedLayer", "services/other",
		"",
		"",
		"",
		"approve",
	)

	if !result.Approved {
		t.Fatalf("expected Approved = true, got false; output:\n%s", out)
	}
	if !strings.Contains(out, "apps/api") || !strings.Contains(out, "apps/web") {
		t.Fatalf("expected the printed candidate to include the selected roots, got:\n%s", out)
	}

	t.Run("layer libs matches libs/shared and not string-prefix sibling libsx/legacy", func(t *testing.T) {
		body_projectConfigAuthoringPart2Test_layerLibsMatchesLibsSharedAndNotStringPrefixSibl_36(t, out)
	})

	t.Run("layer apiLayer lists only apps/api", func(t *testing.T) {
		body_projectConfigAuthoringPart2Test_layerApiLayerListsOnlyAppsApi_49(t, out)
	})

	t.Run("layer unusedLayer matches no discovered directory", func(t *testing.T) {
		body_projectConfigAuthoringPart2Test_layerUnusedLayerMatchesNoDiscoveredDirectory_62(t, out)
	})

	t.Run("uncovered line lists apps/web and libsx/legacy only", func(t *testing.T) {
		body_projectConfigAuthoringPart2Test_uncoveredLineListsAppsWebAndLibsxLegacyOnly_72(t, out)
	})
}

func body_projectConfigAuthoringPart2Test_completeCandidateAndGateOrder_57(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, out := runAuthoring(discovered,
		"1,2",
		"apiLayer", "apps/api",
		"libsLayer", "libs",
		"",
		"apiLayer", "libsLayer",
		"libsLayer", "apiLayer",
		"",
		"apiLayer",
		"approve",
	)

	if !result.Approved {
		t.Fatalf("expected Approved = true, got false; output:\n%s", out)
	}

	t.Run("prints every collected field in the candidate summary", func(t *testing.T) {
		body_projectConfigAuthoringPart2Test_printsEveryCollectedFieldInTheCandidateSummary_133(t, out)
	})

	t.Run("prints candidate summary, then coverage preview, then approval prompt", func(t *testing.T) {
		body_projectConfigAuthoringPart2Test_printsCandidateSummaryThenCoveragePreviewThenApp_167(t, out)
	})
}
