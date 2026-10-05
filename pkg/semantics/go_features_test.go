package semantics

import (
	"testing"
)

// AC-3.3: computeGoFeatures must count exactly one StructuralMetrics field per
// tracked Tree-sitter node kind, in a single traversal. The fixture below
// hand-counts to 2 ifs, 1 for, 1 expression switch, 1 type switch, 1
// select, 2 functions (Foo, Bar), and 1 method (on *T) -- every field must
// match that exact count, not merely be non-zero.
func TestMetrics_CountsEachNodeKindExactly(t *testing.T) {
	source := []byte(`package main

type T struct{}

func Foo(x int) {
	if x > 0 {
	}
	if x < 0 {
	}
	for i := 0; i < x; i++ {
	}
	switch x {
	case 1:
	}
	var v interface{} = x
	switch v.(type) {
	case int:
	}
	ch := make(chan int)
	select {
	case <-ch:
	}
}

func Bar() {
}

func (t *T) Method() {
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	metrics, _ := computeGoFeatures(root, source)

	want := StructuralMetrics{
		Ifs:          2,
		Fors:         1,
		ExprSwitches: 1,
		TypeSwitches: 1,
		Selects:      1,
		Functions:    2,
		Methods:      1,
	}
	if metrics.Ifs != want.Ifs {
		t.Errorf("computeGoFeatures Ifs: got %d, want %d", metrics.Ifs, want.Ifs)
	}
	if metrics.Fors != want.Fors {
		t.Errorf("computeGoFeatures Fors: got %d, want %d", metrics.Fors, want.Fors)
	}
	if metrics.ExprSwitches != want.ExprSwitches {
		t.Errorf("computeGoFeatures ExprSwitches: got %d, want %d", metrics.ExprSwitches, want.ExprSwitches)
	}
	if metrics.TypeSwitches != want.TypeSwitches {
		t.Errorf("computeGoFeatures TypeSwitches: got %d, want %d", metrics.TypeSwitches, want.TypeSwitches)
	}
	if metrics.Selects != want.Selects {
		t.Errorf("computeGoFeatures Selects: got %d, want %d", metrics.Selects, want.Selects)
	}
	if metrics.Functions != want.Functions {
		t.Errorf("computeGoFeatures Functions: got %d, want %d", metrics.Functions, want.Functions)
	}
	if metrics.Methods != want.Methods {
		t.Errorf("computeGoFeatures Methods: got %d, want %d", metrics.Methods, want.Methods)
	}
}

// AC-3.4: max_nesting_depth is the maximum depth of nested block nodes
// within any single function/method body, where the body's own top-level
// block counts as depth 1. This fixture nests: func body (block, depth 1)
// -> if consequence (block, depth 2) -> for body (block, depth 3) -> if
// consequence (block, depth 4). The innermost if body is empty, so it adds
// no further depth.
func TestMetrics_MaxNestingDepthWithinFunctionBody(t *testing.T) {
	source := []byte(`package main

func f() {
	if true {
		for {
			if true {
			}
		}
	}
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	metrics, _ := computeGoFeatures(root, source)

	if metrics.MaxNestingDepth != 4 {
		t.Errorf("computeGoFeatures MaxNestingDepth for 4-deep nested blocks %q: got %d, want %d", source, metrics.MaxNestingDepth, 4)
	}
}

// AC-3.4: a file with no function/method declarations at all must report
// MaxNestingDepth == 0, since there is no function body to measure nesting
// within.
func TestMetrics_ZeroNestingDepthWhenFileHasNoFunctions(t *testing.T) {
	source := []byte(`package main

var x int

type T struct {
	Field int
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	metrics, _ := computeGoFeatures(root, source)

	if metrics.MaxNestingDepth != 0 {
		t.Errorf("computeGoFeatures MaxNestingDepth for a file with no functions %q: got %d, want 0", source, metrics.MaxNestingDepth)
	}
}
