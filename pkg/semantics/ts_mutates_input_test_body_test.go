package semantics

import (
	"testing"
)

type tsMethodCallCountCase struct {
	name      string
	source    string
	wantCount int
}

func expectMutatingMethodCallFindingCount(t *testing.T, tt tsMethodCallCountCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))
	var got []Finding
	for _, f := range findings {
		if f.Kind == "mutates_input" {
			got = append(got, f)
		}
	}
	if len(got) != tt.wantCount {
		t.Fatalf("computeTSFeatures for %q: got %d mutates_input findings (%+v), want %d", tt.source, len(got), findings, tt.wantCount)
	}
}
