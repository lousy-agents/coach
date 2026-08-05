package semantics

import (
	"testing"
)

// AC-R4.1: computeTSFeatures must count exactly one StructuralMetrics field
// per tracked node kind, with TypeSwitches/Selects always 0 (no TS analog).
// The fixture hand-counts to 2 ifs, 1 for, 1 for...of, 1 switch, 1 top-level
// function, and 1 class method.
func TestComputeTSFeatures_CountsEachNodeKindExactly(t *testing.T) {
	source := []byte(`function f(x: number) {
	if (x > 0) {
	}
	if (x < 0) {
	}
	for (let i = 0; i < x; i++) {
	}
	for (const v of [1, 2, 3]) {
	}
	switch (x) {
		case 1:
			break;
	}
}

class C {
	method() {
	}
}
`)
	root, closeTree := mustParseTS(t, source)
	defer closeTree()

	metrics, _ := computeTSFeatures(root, source)

	want := StructuralMetrics{
		Ifs:          2,
		Fors:         2,
		ExprSwitches: 1,
		Functions:    1,
		Methods:      1,
	}
	if metrics.Ifs != want.Ifs {
		t.Errorf("computeTSFeatures Ifs: got %d, want %d", metrics.Ifs, want.Ifs)
	}
	if metrics.Fors != want.Fors {
		t.Errorf("computeTSFeatures Fors: got %d, want %d", metrics.Fors, want.Fors)
	}
	if metrics.ExprSwitches != want.ExprSwitches {
		t.Errorf("computeTSFeatures ExprSwitches: got %d, want %d", metrics.ExprSwitches, want.ExprSwitches)
	}
	if metrics.Functions != want.Functions {
		t.Errorf("computeTSFeatures Functions: got %d, want %d", metrics.Functions, want.Functions)
	}
	if metrics.Methods != want.Methods {
		t.Errorf("computeTSFeatures Methods: got %d, want %d", metrics.Methods, want.Methods)
	}
	if metrics.TypeSwitches != 0 {
		t.Errorf("computeTSFeatures TypeSwitches: got %d, want 0 (no TypeScript analog)", metrics.TypeSwitches)
	}
	if metrics.Selects != 0 {
		t.Errorf("computeTSFeatures Selects: got %d, want 0 (no TypeScript analog)", metrics.Selects)
	}
}

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
			root, closeTree := mustParseTS(t, []byte(tt.source))
			defer closeTree()

			metrics, _ := computeTSFeatures(root, []byte(tt.source))

			if metrics.MaxNestingDepth != tt.want {
				t.Errorf("computeTSFeatures MaxNestingDepth for %q: got %d, want %d", tt.source, metrics.MaxNestingDepth, tt.want)
			}
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

// AC-R2.3 (metrics half): computeTSFeatures must behave identically when
// given a TSX-parsed tree, proving the shared extractor works across both
// grammars (Node.Kind() strings, which the walk matches on, resolve
// against each node's own tree language regardless of which grammar parsed
// it).
func TestComputeTSFeatures_WorksOnTSXParsedTree(t *testing.T) {
	source := []byte(`const App = () => {
	if (true) {
		return null;
	}
	return null;
};
`)
	root, closeTree := mustParseTSX(t, source)
	defer closeTree()

	metrics, _ := computeTSFeatures(root, source)

	if metrics.Functions != 1 {
		t.Errorf("computeTSFeatures on TSX tree for %q: Functions = %d, want 1", source, metrics.Functions)
	}
	if metrics.Ifs != 1 {
		t.Errorf("computeTSFeatures on TSX tree for %q: Ifs = %d, want 1", source, metrics.Ifs)
	}
	if metrics.MaxNestingDepth != 2 {
		t.Errorf("computeTSFeatures on TSX tree for %q: MaxNestingDepth = %d, want 2", source, metrics.MaxNestingDepth)
	}
}
