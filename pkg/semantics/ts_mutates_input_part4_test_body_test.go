package semantics

import (
	"testing"
)

func body_tsMutatesInputPart4Test_104(t *testing.T, tt struct {
	name   string
	source string
}) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))

	for _, f := range findings {
		if f.Kind == "mutates_input" {
			t.Fatalf("computeTSFeatures for %q: got mutates_input finding %+v, want none because the write targets a binding that shadows the parameter", tt.source, f)
		}
	}
}

func body_tsMutatesInputPart4Test_147(t *testing.T, tt struct {
	name   string
	source string
}) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))
	for _, f := range findings {
		if f.Kind == "mutates_input" {
			t.Fatalf("computeTSFeatures for %q: got mutates_input finding %+v, want none (non-identifier-bound parameter must never be tracked)", tt.source, f)
		}
	}
}

func body_tsMutatesInputPart4Test_192(t *testing.T, tt struct {
	name     string
	source   string
	wantName string
	evidence string
}) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))
	got := mustFindTSMutatesInput(t, tt.source, findings)

	if got.Name != tt.wantName {
		t.Errorf("Finding.Name = %q, want %q", got.Name, tt.wantName)
	}
	gotText := tt.source[got.Location.StartByte:got.Location.EndByte]
	if gotText != tt.evidence {
		t.Errorf("Finding.Location text = %q, want %q", gotText, tt.evidence)
	}
}
