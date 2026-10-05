package semantics

import (
	"testing"
)

type tsMutatesInputSourceCase struct {
	name   string
	source string
}

func expectNoFindingAfterParameterRebind(t *testing.T, tt tsMutatesInputSourceCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))

	for _, f := range findings {
		if f.Kind == "mutates_input" {
			t.Fatalf("computeTSFeatures for %q: got mutates_input finding %+v, want none after parameter rebinding", tt.source, f)
		}
	}
}

func expectNoFindingForBlockLocalBinding(t *testing.T, tt tsMutatesInputSourceCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))

	for _, f := range findings {
		if f.Kind == "mutates_input" {
			t.Fatalf("computeTSFeatures for %q: got mutates_input finding %+v, want none because the write targets a block-local binding", tt.source, f)
		}
	}
}

type tsMutatesInputFindingCase struct {
	name     string
	source   string
	wantName string
	evidence string
}

func expectCompoundAssignmentFinding(t *testing.T, tt tsMutatesInputFindingCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))

	got := mutatesInputFindingNamed(findings, tt.wantName, tt.evidence)
	if got == nil {
		t.Fatalf("computeTSFeatures for compound assignment %q: want mutates_input named %q with evidence %q, got %+v", tt.source, tt.wantName, tt.evidence, findings)
	}
}
