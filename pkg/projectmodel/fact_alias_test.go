package projectmodel

import (
	"encoding/json"
	"testing"
)

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
