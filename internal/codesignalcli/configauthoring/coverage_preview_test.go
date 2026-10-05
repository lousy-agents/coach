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

func uncoveredLineListsAppsWebLibsxLegacyOnly(t *testing.T, out string) {
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

func printsEveryCollectedFieldCandidateSummary(t *testing.T, out string) {
	rootsLine := lineContaining(out, "roots:")
	if rootsLine == "" {
		t.Fatalf("expected the candidate summary's roots line, got:\n%s", out)
	}
	if !strings.Contains(rootsLine, "apps/api") || !strings.Contains(rootsLine, "apps/web") {
		t.Fatalf("expected the candidate summary's roots line to list the selected roots, got:\n%s", rootsLine)
	}

	if lineContaining(out, "forbidden_imports:") == "" {
		t.Fatalf("expected the candidate summary's forbidden_imports header line, got:\n%s", out)
	}
	if lineContaining(out, "apiLayer -> libsLayer") == "" {
		t.Fatalf("expected the candidate summary to print the declared forbidden pair apiLayer -> libsLayer, got:\n%s", out)
	}
	if lineContaining(out, "libsLayer -> apiLayer") == "" {
		t.Fatalf("expected the candidate summary to print the declared forbidden pair libsLayer -> apiLayer, got:\n%s", out)
	}
	if lineContaining(out, "- apiLayer: apps/api") == "" {
		t.Fatalf("expected the candidate summary's layers block to list apiLayer with its prefix, got:\n%s", out)
	}
	if lineContaining(out, "- libsLayer: libs") == "" {
		t.Fatalf("expected the candidate summary's layers block to list libsLayer with its prefix, got:\n%s", out)
	}

	requiredLayerLine := lineContaining(out, "required_layer:")
	if requiredLayerLine == "" {
		t.Fatalf("expected the candidate summary's required_layer line, got:\n%s", out)
	}
	if !strings.Contains(requiredLayerLine, "apiLayer") {
		t.Fatalf("expected the candidate summary's required_layer line to name apiLayer, got:\n%s", requiredLayerLine)
	}
}

func printsCandidateSummaryThenCoveragePreviewThenApproval(t *testing.T, out string) {
	candidateIdx := strings.Index(out, "Candidate project config:")
	coverageIdx := strings.Index(out, "Coverage preview:")
	approvalIdx := strings.Index(out, "Type 'approve'")
	if candidateIdx == -1 || coverageIdx == -1 || approvalIdx == -1 {
		t.Fatalf("expected all three markers (candidate summary, coverage preview, approval prompt) to appear, got indices %d/%d/%d, output:\n%s", candidateIdx, coverageIdx, approvalIdx, out)
	}
	if !(candidateIdx < coverageIdx && coverageIdx < approvalIdx) {
		t.Fatalf("expected candidate summary, then coverage preview, then approval prompt in that order (got indices %d, %d, %d), output:\n%s", candidateIdx, coverageIdx, approvalIdx, out)
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

func completeCandidateGateOrder(t *testing.T, discovered projectmodel.TSRootDiscoveryResult) {
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
		printsEveryCollectedFieldCandidateSummary(t, out)
	})

	t.Run("prints candidate summary, then coverage preview, then approval prompt", func(t *testing.T) {
		printsCandidateSummaryThenCoveragePreviewThenApproval(t, out)
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
