package configauthoring

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func coveragePreviewMixedLayerMatches(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
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
		layerLibsMatchesLibsSharedNotStringPrefix(t, out)
	})

	t.Run("layer apiLayer lists only apps/api", func(t *testing.T) {
		layerApiLayerListsOnlyAppsApi(t, out)
	})

	t.Run("layer unusedLayer matches no discovered directory", func(t *testing.T) {
		layerUnusedLayerMatchesNoDiscoveredDirectory(t, out)
	})

	t.Run("uncovered line lists apps/web and libsx/legacy only", func(t *testing.T) {
		uncoveredLineListsAppsWebLibsxLegacyOnly(t, out)
	})
}

func layerLibsMatchesLibsSharedNotStringPrefix(t *testing.T, out string) {
	libsLine := lineContaining(out, `layer "libsLayer" (prefixes:`)
	if libsLine == "" {
		t.Fatalf("expected a coverage line for layer libsLayer, got:\n%s", out)
	}
	if !strings.Contains(libsLine, "libs/shared") {
		t.Fatalf("expected layer libsLayer's coverage line to list its matching discovered directory, got:\n%s", libsLine)
	}
	if strings.Contains(libsLine, "libsx/legacy") {
		t.Fatalf("expected layer libsLayer's coverage line NOT to list libsx/legacy (a string-prefix sibling of prefix %q, not a path-segment match), got:\n%s", "libs", libsLine)
	}
}

func layerApiLayerListsOnlyAppsApi(t *testing.T, out string) {
	apiLine := lineContaining(out, `layer "apiLayer" (prefixes:`)
	if apiLine == "" {
		t.Fatalf("expected a coverage line for layer apiLayer, got:\n%s", out)
	}
	if !strings.Contains(apiLine, "apps/api") {
		t.Fatalf("expected layer apiLayer's coverage line to list its matching discovered directory, got:\n%s", apiLine)
	}
	if strings.Contains(apiLine, "apps/web") || strings.Contains(apiLine, "libs") {
		t.Fatalf("expected layer apiLayer's coverage line to list only apps/api, got:\n%s", apiLine)
	}
}

func layerUnusedLayerMatchesNoDiscoveredDirectory(t *testing.T, out string) {
	unusedLine := lineContaining(out, `layer "unusedLayer" (prefixes:`)
	if unusedLine == "" {
		t.Fatalf("expected a coverage line for layer unusedLayer, got:\n%s", out)
	}
	if strings.Contains(unusedLine, "apps/") || strings.Contains(unusedLine, "libs") {
		t.Fatalf("expected layer unusedLayer's coverage line to match no discovered directory, got:\n%s", unusedLine)
	}
}
