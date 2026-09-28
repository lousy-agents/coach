package semantics

import (
	"testing"
)

func body_tsMutatesInputPart3Test_81(t *testing.T, tt struct {
	name   string
	source string
}) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))

	for _, f := range findings {
		if f.Kind == "mutates_input" {
			t.Fatalf("computeTSFeatures for %q: got mutates_input finding %+v, want none after parameter rebinding", tt.source, f)
		}
	}
}

func body_tsMutatesInputPart3Test_148(t *testing.T, tt struct {
	name   string
	source string
}) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))

	for _, f := range findings {
		if f.Kind == "mutates_input" {
			t.Fatalf("computeTSFeatures for %q: got mutates_input finding %+v, want none because the write targets a block-local binding", tt.source, f)
		}
	}
}

func body_tsMutatesInputPart3Test_200(t *testing.T, tt struct {
	name     string
	source   string
	wantName string
	evidence string
}) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))

	got := mutatesInputFindingNamed(findings, tt.wantName, tt.evidence)
	if got == nil {
		t.Fatalf("computeTSFeatures for compound assignment %q: want mutates_input named %q with evidence %q, got %+v", tt.source, tt.wantName, tt.evidence, findings)
	}
}
