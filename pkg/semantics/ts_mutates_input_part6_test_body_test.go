package semantics

import (
	"testing"
)

func expectTSUpdateExpressionFinding(t *testing.T, tt tsMutatesInputFindingCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))

	got := mutatesInputFindingNamed(findings, tt.wantName, tt.evidence)
	if got == nil {
		t.Fatalf("computeTSFeatures for update expression %q: want a mutates_input finding named %q with evidence %q, got %+v", tt.source, tt.wantName, tt.evidence, findings)
	}
}

func expectDestructuringAssignmentFinding(t *testing.T, tt tsMutatesInputFindingCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))
	got := mutatesInputFindingNamed(findings, tt.wantName, tt.evidence)
	if got == nil {
		t.Fatalf("computeTSFeatures for destructuring assignment %q: want mutates_input named %q with evidence %q, got %+v", tt.source, tt.wantName, tt.evidence, findings)
	}
}
