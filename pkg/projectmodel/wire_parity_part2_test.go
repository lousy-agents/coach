package projectmodel

import (
	"encoding/json"

	"testing"
)

// TestReachabilityGapDiagnosticCodeParity guards issue #216's coverage-honesty
// invariant: tsReachabilityGapDiagnosticCodes (ts_reachability.go) must list
// exactly the same diagnostic codes as js/semantics/src/project-sidecar/
// reachability-registry.ts's GAP_* constants. Go cannot import that
// TypeScript file, so it is read as source text (mirroring
// TestProtocolGoTSFieldParity's approach for protocol.ts) -- a code present
// on only one side means BuildTypeScriptReachability/BuildTypeScriptLayerBypass
// either silently report Coverage.Complete: true for a genuinely unverified
// hop (a code missing from the Go side) or over-report incompleteness for a
// hop that was actually fully resolved (a code missing from the TS side).
func TestReachabilityGapDiagnosticCodeParity(t *testing.T) {
	tsSource := readReachabilityRegistryTSSource(t)
	tsCodes := gapCodeConstPattern.FindAllSubmatch(tsSource, -1)
	if len(tsCodes) == 0 {
		t.Fatal("reachability-registry.ts: no GAP_* constants found; gapCodeConstPattern likely no longer matches the source")
	}

	tsSet := make(map[string]bool, len(tsCodes))
	for _, m := range tsCodes {
		tsSet[string(m[1])] = true
	}

	for code := range tsReachabilityGapDiagnosticCodes {
		if !tsSet[code] {
			t.Errorf("tsReachabilityGapDiagnosticCodes (ts_reachability.go) has %q, but reachability-registry.ts has no matching GAP_* constant", code)
		}
	}
	for code := range tsSet {
		if !tsReachabilityGapDiagnosticCodes[code] {
			t.Errorf("reachability-registry.ts declares GAP_* constant %q, but tsReachabilityGapDiagnosticCodes (ts_reachability.go) does not include it", code)
		}
	}
}

// TestRootScopePathInvariants guards RootScope's per-file identity contract
// (issue #331 review findings REV-386-01/02) across a JSON round trip:
// len(AnalyzedPaths) must equal AnalyzedFiles, and AnalyzedPaths plus
// UnanalyzedPaths must together equal CandidateFiles.
func TestRootScopePathInvariants(t *testing.T) {
	model := Model{
		RootScopes: []RootScope{
			{
				Root:            ".",
				CandidateFiles:  3,
				AnalyzedFiles:   2,
				AnalyzedPaths:   []string{"a.ts", "b.ts"},
				UnanalyzedPaths: []string{"c.tsx"},
			},
		},
		Coverage: Coverage{Phase: "test"},
	}

	data, err := json.Marshal(model)
	if err != nil {
		t.Fatalf("marshaling model: %s", err)
	}

	var decoded Model
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshaling model: %s", err)
	}
	if len(decoded.RootScopes) != 1 {
		t.Fatalf("expected exactly one decoded root scope, got %+v", decoded.RootScopes)
	}

	got := decoded.RootScopes[0]
	if len(got.AnalyzedPaths) != got.AnalyzedFiles {
		t.Errorf("len(analyzed_paths)=%d does not equal analyzed_files=%d, got %+v", len(got.AnalyzedPaths), got.AnalyzedFiles, got)
	}
	if len(got.AnalyzedPaths)+len(got.UnanalyzedPaths) != got.CandidateFiles {
		t.Errorf("len(analyzed_paths)+len(unanalyzed_paths)=%d does not equal candidate_files=%d, got %+v", len(got.AnalyzedPaths)+len(got.UnanalyzedPaths), got.CandidateFiles, got)
	}
}
