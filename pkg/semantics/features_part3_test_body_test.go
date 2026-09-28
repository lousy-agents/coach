package semantics

import (
	"testing"
)

func body_featuresPart3Test_81(t *testing.T, tt struct {
	name     string
	source   string
	wantName string
	evidence string
}) {
	root, closeTree := mustParseGo(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeGoFeatures(root, []byte(tt.source))

	got := mutatesInputFinding(findings, tt.wantName)
	if got == nil {
		t.Fatalf("computeGoFeatures findings for %q: want a mutates_input finding named %q, got %+v", tt.source, tt.wantName, findings)
	}
	if got.Evidence != tt.evidence {
		t.Errorf("mutates_input Evidence: got %q, want %q", got.Evidence, tt.evidence)
	}
}

func body_featuresPart3Test_134(t *testing.T, tt struct {
	name     string
	source   string
	wantName string
	evidence string
}) {
	root, closeTree := mustParseGo(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeGoFeatures(root, []byte(tt.source))

	got := mutatesInputFinding(findings, tt.wantName)
	if got == nil {
		t.Fatalf("computeGoFeatures findings for update expression %q: want a mutates_input finding named %q, got %+v", tt.source, tt.wantName, findings)
	}
	if got.Evidence != tt.evidence {
		t.Errorf("mutates_input Evidence for update expression: got %q, want %q", got.Evidence, tt.evidence)
	}
}
