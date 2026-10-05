package configauthoring

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func TestAuthorProjectConfig_CoveragePreviewShowsCandidateAndDirectoryCoverage(t *testing.T) {
	discovered := projectmodel.TSRootDiscoveryResult{
		Roots:      []string{"apps/api", "apps/web"},
		Candidates: []string{"libs/shared", "libsx/legacy"},
		Complete:   true,
	}

	t.Run("mixed layer matches", func(t *testing.T) {
		coveragePreviewMixedLayerMatches(t, discovered)
	})

	t.Run("no layers declared at all: every discovered directory is shown as uncovered", func(t *testing.T) {
		noLayersDeclaredAllEveryDiscoveredDirectoryShown(t, discovered)
	})

	t.Run("complete candidate and gate order", func(t *testing.T) {
		completeCandidateGateOrder(t, discovered)
	})

	t.Run("a layer whose prefix is the universal root \".\" matches every discovered directory and leaves nothing uncovered", func(t *testing.T) {
		aLayerWhosePrefixUniversalRootMatchesEvery(t, discovered)
	})
}

func noLayersDeclaredAllEveryDiscoveredDirectoryShown(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, out := runAuthoring(discovered,
		"1,2",
		"",
		"",
		"",
		"approve",
	)

	if !result.Approved {
		t.Fatalf("expected Approved = true, got false")
	}
	if len(result.Layers) != 0 {
		t.Fatalf("expected no layers to be declared, got %+v", result.Layers)
	}

	uncoveredLine := lineContaining(out, "no declared layer matches")
	if uncoveredLine == "" {
		t.Fatalf("expected an uncovered-directories line, got:\n%s", out)
	}
	for _, dir := range []string{"apps/api", "apps/web", "libs/shared", "libsx/legacy"} {
		if !strings.Contains(uncoveredLine, dir) {
			t.Fatalf("expected discovered directory %q to be listed as uncovered when no layers are declared, got:\n%s", dir, uncoveredLine)
		}
	}
}

func aLayerWhosePrefixUniversalRootMatchesEvery(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
	result, out := runAuthoring(discovered,
		"1,2",
		"everything", ".",
		"",
		"",
		"",
		"approve",
	)

	if !result.Approved {
		t.Fatalf("expected Approved = true, got false; output:\n%s", out)
	}

	everythingLine := lineContaining(out, `layer "everything" (prefixes:`)
	if everythingLine == "" {
		t.Fatalf("expected a coverage line for layer everything, got:\n%s", out)
	}
	for _, dir := range []string{"apps/api", "apps/web", "libs/shared", "libsx/legacy"} {
		if !strings.Contains(everythingLine, dir) {
			t.Fatalf("expected the \".\" prefix to match every discovered directory, missing %q, got:\n%s", dir, everythingLine)
		}
	}

	uncoveredLine := lineContaining(out, "no declared layer matches")
	if uncoveredLine == "" {
		t.Fatalf("expected an uncovered-directories line, got:\n%s", out)
	}
	if !strings.Contains(uncoveredLine, "(none)") {
		t.Fatalf("expected no discovered directory to be left uncovered when a layer's prefix is \".\", got:\n%s", uncoveredLine)
	}
}
