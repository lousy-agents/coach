package projectmodel

import "testing"

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
