package semantics

import (
	"testing"
)

func expectNoFindingForPredeclaredLocal(t *testing.T, tt tsMutatesInputSourceCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))

	for _, f := range findings {
		if f.Kind == "mutates_input" {
			t.Fatalf("computeTSFeatures for predeclared local shadowing %q: got mutates_input finding %+v, want none because the write targets a local binding, not the parameter", tt.source, f)
		}
	}
}

func expectNoFindingForDestructuringRead(t *testing.T, tt tsMutatesInputSourceCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))
	for _, f := range findings {
		if f.Kind == "mutates_input" {
			t.Fatalf("computeTSFeatures for destructuring read %q: got mutates_input finding %+v, want none because p.x is read but not assigned", tt.source, f)
		}
	}
}

func expectFindingDespiteDestructuringAlias(t *testing.T, tt tsMutatesInputSourceCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))
	got := mustFindTSMutatesInput(t, tt.source, findings)

	if got.Name != "f:p" {
		t.Errorf("Finding.Name for %q = %q, want %q", tt.source, got.Name, "f:p")
	}
}
