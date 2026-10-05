package codesignalcli

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func body_projectConfigAuthoringPart2Test_aLayerWhosePrefixIsTheUniversalRootMatchesEveryD_180(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
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
