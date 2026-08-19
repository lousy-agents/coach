package projectmodel

import (
	"os"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"
)

// TestCallGraphAlgorithmVersionMatchesGoMod guards CallGraphAlgorithm's
// pinned golang.org/x/tools version suffix against drifting from go.mod
// (which renovate.json auto-merges), so the provenance string it embeds in
// every CallGraphResult stays accurate.
func TestCallGraphAlgorithmVersionMatchesGoMod(t *testing.T) {
	data, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatalf("reading repository go.mod: %v", err)
	}
	mf, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		t.Fatalf("parsing repository go.mod: %v", err)
	}

	var pinnedVersion string
	for _, req := range mf.Require {
		if req.Mod.Path == "golang.org/x/tools" {
			pinnedVersion = req.Mod.Version
			break
		}
	}
	if pinnedVersion == "" {
		t.Fatal("golang.org/x/tools requirement not found in go.mod")
	}

	want := "golang.org/x/tools@" + pinnedVersion
	if !strings.HasSuffix(CallGraphAlgorithm, want) {
		t.Errorf("CallGraphAlgorithm %q does not reflect go.mod's pinned %q; update the const when x/tools is bumped", CallGraphAlgorithm, want)
	}
}

// TestCallSiteDiagnosticCountsMatchesClassification guards
// callSiteDiagnosticCounts against drifting from the diagnostic codes
// classifyCallSite actually emits and the counts keys BuildGoCallGraph
// initializes. A code present in one but not the other would silently
// index Coverage.Counts with the zero-value "" key (a map miss) instead
// of failing loudly.
func TestCallSiteDiagnosticCountsMatchesClassification(t *testing.T) {
	wantCodes := map[string]bool{
		DiagCallUnresolvedInterface:             true,
		DiagCallUnresolvedFunctionValue:         true,
		DiagCallUnresolvedReflection:            true,
		DiagCallUnresolvedFrameworkRegistration: true,
		DiagCallUnresolvedSyntheticWrapper:      true,
	}
	if len(callSiteDiagnosticCounts) != len(wantCodes) {
		t.Fatalf("callSiteDiagnosticCounts has %d entries, want %d matching the classifyCallSite diagnostic codes", len(callSiteDiagnosticCounts), len(wantCodes))
	}

	initializedCounts := map[string]bool{
		"unresolved_interface":              true,
		"unresolved_function_value":         true,
		"unresolved_reflection":             true,
		"unresolved_framework_registration": true,
		"unresolved_synthetic_wrapper":      true,
	}
	for code, countKey := range callSiteDiagnosticCounts {
		if !wantCodes[code] {
			t.Errorf("callSiteDiagnosticCounts has unexpected diagnostic code %q", code)
		}
		if countKey == "" || !initializedCounts[countKey] {
			t.Errorf("callSiteDiagnosticCounts[%q] = %q, want a key BuildGoCallGraph's counts literal initializes", code, countKey)
		}
	}
}
