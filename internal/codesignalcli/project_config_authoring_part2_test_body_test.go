package codesignalcli

import (
	"strings"
	"testing"
)

func body_projectConfigAuthoringPart2Test_layerLibsMatchesLibsSharedAndNotStringPrefixSibl_36(t *testing.T, out string) {
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

func body_projectConfigAuthoringPart2Test_layerApiLayerListsOnlyAppsApi_49(t *testing.T, out string) {
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

func body_projectConfigAuthoringPart2Test_layerUnusedLayerMatchesNoDiscoveredDirectory_62(t *testing.T, out string) {
	unusedLine := lineContaining(out, `layer "unusedLayer" (prefixes:`)
	if unusedLine == "" {
		t.Fatalf("expected a coverage line for layer unusedLayer, got:\n%s", out)
	}
	if strings.Contains(unusedLine, "apps/") || strings.Contains(unusedLine, "libs") {
		t.Fatalf("expected layer unusedLayer's coverage line to match no discovered directory, got:\n%s", unusedLine)
	}
}
