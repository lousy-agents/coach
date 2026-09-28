package semantics

import (
	"testing"
)

func body_featuresPart5Test_113(t *testing.T, tt struct {
	name   string
	source string
}) {
	source := []byte(tt.source)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if hasFinding(findings, "mutates_input", "f:cfg") {
		t.Errorf("computeGoFeatures findings for closure-shadowed parameter %q: want no mutates_input finding named %q (closure's cfg is a distinct binding), got %+v", source, "f:cfg", findings)
	}
}

func body_featuresPart5Test_213(t *testing.T, tt struct {
	name   string
	source string
}) {
	source := []byte(tt.source)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if hasFinding(findings, "mutates_input", "f:cfg") {
		t.Errorf("computeGoFeatures findings for %q: want no mutates_input finding for shadowed parameter %q, got %+v", tt.source, "f:cfg", findings)
	}
}
