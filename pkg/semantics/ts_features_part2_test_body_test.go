package semantics

import (
	"testing"
)

type tsMaxNestingDepthCase struct {
	name   string
	source string
	want   int
}

func expectTSMaxNestingDepth(t *testing.T, tt tsMaxNestingDepthCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	metrics, _ := computeTSFeatures(root, []byte(tt.source))

	if metrics.MaxNestingDepth != tt.want {
		t.Errorf("computeTSFeatures MaxNestingDepth for %q: got %d, want %d", tt.source, metrics.MaxNestingDepth, tt.want)
	}
}
