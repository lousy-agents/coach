package semantics

import (
	"testing"
)

// AC-R4.2: MaxNestingDepth counts one level per braced statement_block
// within a function body (the body's own braces are depth 1), and a
// brace-less body contributes no additional depth.
func TestComputeTSFeatures_MaxNestingDepth(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   int
	}{
		{
			name:   "two braced nested ifs",
			source: "function f() { if (a) { if (b) { } } }",
			want:   3,
		},
		{
			name:   "brace-less inner if",
			source: "function f() { if (a) { if (b) g(); } }",
			want:   2,
		},
		{
			name:   "arrow function with expression body contributes zero depth",
			source: "const f = (x: number) => x + 1;",
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_tsFeaturesPart2Test_34(t, tt)
		})
	}
}

// AC-R4.5: a file whose only function-like node is an arrow_function must
// report Functions == 1 and Methods == 0, confirming arrows count toward
// Functions.
func TestComputeTSFeatures_ArrowFunctionCountsAsFunctionNotMethod(t *testing.T) {
	source := []byte(`const f = (x: number) => { return x; };`)
	root, closeTree := mustParseTS(t, source)
	defer closeTree()

	metrics, _ := computeTSFeatures(root, source)

	if metrics.Functions != 1 {
		t.Errorf("computeTSFeatures for arrow-only file %q: Functions = %d, want 1", source, metrics.Functions)
	}
	if metrics.Methods != 0 {
		t.Errorf("computeTSFeatures for arrow-only file %q: Methods = %d, want 0", source, metrics.Methods)
	}
}
