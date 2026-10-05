package codesignalcli

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_projectConfigAuthoringPart2Test_uncoveredLineListsAppsWebAndLibsxLegacyOnly_72(t *testing.T, out string) {
	uncoveredLine := lineContaining(out, "no declared layer matches")
	if uncoveredLine == "" {
		t.Fatalf("expected an uncovered-directories line, got:\n%s", out)
	}
	if !strings.Contains(uncoveredLine, "apps/web") {
		t.Fatalf("expected apps/web (matched by no layer) to be listed as uncovered, got:\n%s", uncoveredLine)
	}
	if !strings.Contains(uncoveredLine, "libsx/legacy") {
		t.Fatalf("expected libsx/legacy (a string-prefix sibling of libsLayer's prefix %q, not a real path-segment match) to be listed as uncovered, got:\n%s", "libs", uncoveredLine)
	}
	if strings.Contains(uncoveredLine, "apps/api") || strings.Contains(uncoveredLine, "libs/shared") {
		t.Fatalf("expected only apps/web and libsx/legacy to be listed as uncovered, got:\n%s", uncoveredLine)
	}
}

func body_projectConfigAuthoringPart2Test_noLayersDeclaredAtAllEveryDiscoveredDirectoryIsS_89(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
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
