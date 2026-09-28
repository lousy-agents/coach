package semantics

import (
	"testing"
)

func body_tsMutatesInputPart6Test_61(t *testing.T, tt struct {
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
		t.Fatalf("computeTSFeatures for update expression %q: want a mutates_input finding named %q with evidence %q, got %+v", tt.source, tt.wantName, tt.evidence, findings)
	}
}

func body_tsMutatesInputPart6Test_149(t *testing.T, tt struct {
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
		t.Fatalf("computeTSFeatures for destructuring assignment %q: want mutates_input named %q with evidence %q, got %+v", tt.source, tt.wantName, tt.evidence, findings)
	}
}
