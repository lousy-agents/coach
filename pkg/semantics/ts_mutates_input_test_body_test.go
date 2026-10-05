package semantics

import (
	"testing"
)

func body_tsMutatesInputTest_69(t *testing.T, tt struct {
	name      string
	source    string
	wantCount int
}) {
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
