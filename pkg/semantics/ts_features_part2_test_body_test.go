package semantics

import (
	"testing"
)

func body_tsFeaturesPart2Test_34(t *testing.T, tt struct {
	name   string
	source string
	want   int
}) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	metrics, _ := computeTSFeatures(root, []byte(tt.source))

	if metrics.MaxNestingDepth != tt.want {
		t.Errorf("computeTSFeatures MaxNestingDepth for %q: got %d, want %d", tt.source, metrics.MaxNestingDepth, tt.want)
	}
}
