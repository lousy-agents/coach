package semantics

import (
	"reflect"
	"testing"
)

// AC-3.3: computeFeatures must count exactly one StructuralMetrics field per
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

	metrics, _ := computeFeatures(root, source)

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
		t.Errorf("computeFeatures Ifs: got %d, want %d", metrics.Ifs, want.Ifs)
	}
	if metrics.Fors != want.Fors {
		t.Errorf("computeFeatures Fors: got %d, want %d", metrics.Fors, want.Fors)
	}
	if metrics.ExprSwitches != want.ExprSwitches {
		t.Errorf("computeFeatures ExprSwitches: got %d, want %d", metrics.ExprSwitches, want.ExprSwitches)
	}
	if metrics.TypeSwitches != want.TypeSwitches {
		t.Errorf("computeFeatures TypeSwitches: got %d, want %d", metrics.TypeSwitches, want.TypeSwitches)
	}
	if metrics.Selects != want.Selects {
		t.Errorf("computeFeatures Selects: got %d, want %d", metrics.Selects, want.Selects)
	}
	if metrics.Functions != want.Functions {
		t.Errorf("computeFeatures Functions: got %d, want %d", metrics.Functions, want.Functions)
	}
	if metrics.Methods != want.Methods {
		t.Errorf("computeFeatures Methods: got %d, want %d", metrics.Methods, want.Methods)
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

	metrics, _ := computeFeatures(root, source)

	if metrics.MaxNestingDepth != 4 {
		t.Errorf("computeFeatures MaxNestingDepth for 4-deep nested blocks %q: got %d, want %d", source, metrics.MaxNestingDepth, 4)
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

	metrics, _ := computeFeatures(root, source)

	if metrics.MaxNestingDepth != 0 {
		t.Errorf("computeFeatures MaxNestingDepth for a file with no functions %q: got %d, want 0", source, metrics.MaxNestingDepth)
	}
}

// AC-3.5: a function_declaration whose name matches ^New([A-Z0-9_]|$) must
// emit a "constructor_func" Finding named after the function. NewFoo and the
// bare New both match; Newton must not, since 't' is neither uppercase,
// a digit, underscore, nor end-of-string.
func TestFindings_ConstructorFuncMatchesNewPrefixButNotNewton(t *testing.T) {
	source := []byte(`package main

func NewFoo() {}

func New() {}

func Newton() {}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeFeatures(root, source)

	constructorNames := map[string]bool{}
	for _, f := range findings {
		if f.Kind == "constructor_func" {
			constructorNames[f.Name] = true
		}
	}

	if !constructorNames["NewFoo"] {
		t.Errorf("computeFeatures findings for %q: want a constructor_func finding named %q, got %+v", source, "NewFoo", findings)
	}
	if !constructorNames["New"] {
		t.Errorf("computeFeatures findings for %q: want a constructor_func finding named %q, got %+v", source, "New", findings)
	}
	if constructorNames["Newton"] {
		t.Errorf("computeFeatures findings for %q: want no constructor_func finding named %q (does not match ^New([A-Z0-9_]|$)), got %+v", source, "Newton", findings)
	}
}

// hasFinding reports whether findings contains a Finding with the given
// kind and name.
func hasFinding(findings []Finding, kind, name string) bool {
	for _, f := range findings {
		if f.Kind == kind && f.Name == name {
			return true
		}
	}
	return false
}

// AC-3.6: a function_declaration whose result includes a pointer_type (here,
// the single unnamed return value `*Foo`) must emit a "pointer_return"
// Finding named after the function.
func TestFindings_PointerReturnOnFunction(t *testing.T) {
	source := []byte(`package main

type Foo struct{}

func f() *Foo {
	return nil
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeFeatures(root, source)

	if !hasFinding(findings, "pointer_return", "f") {
		t.Errorf("computeFeatures findings for pointer-returning function %q: want a pointer_return finding named %q, got %+v", source, "f", findings)
	}
}

// AC-3.6: the same pointer_type-in-result rule applies to method_declaration,
// using the method's field_identifier name.
func TestFindings_PointerReturnOnMethod(t *testing.T) {
	source := []byte(`package main

type T struct{}
type Foo struct{}

func (t *T) Method() *Foo {
	return nil
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeFeatures(root, source)

	if !hasFinding(findings, "pointer_return", "Method") {
		t.Errorf("computeFeatures findings for pointer-returning method %q: want a pointer_return finding named %q, got %+v", source, "Method", findings)
	}
}

// AC-3.6: a function returning a plain value type (no pointer_type anywhere
// in the result) must not emit a pointer_return finding.
func TestFindings_ValueReturnEmitsNoPointerFinding(t *testing.T) {
	source := []byte(`package main

type Foo struct{}

func f() Foo {
	return Foo{}
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeFeatures(root, source)

	if hasFinding(findings, "pointer_return", "f") {
		t.Errorf("computeFeatures findings for value-returning function %q: want no pointer_return finding named %q, got %+v", source, "f", findings)
	}
}

// AC-3.7: Finding must carry only grammar-node facts (Kind, Name, Location)
// -- no severity, smell, or advice-shaped fields -- so findings stay
// free of qualitative judgments. Checked structurally via reflection rather
// than by code review, so a future field addition fails this test loudly.
func TestFinding_StructCarriesOnlyDataFields(t *testing.T) {
	typ := reflect.TypeOf(Finding{})

	wantFields := []string{"Kind", "Name", "Location"}
	if typ.NumField() != len(wantFields) {
		t.Fatalf("Finding field count: got %d fields, want exactly %d (%v)", typ.NumField(), len(wantFields), wantFields)
	}
	for i, want := range wantFields {
		if got := typ.Field(i).Name; got != want {
			t.Errorf("Finding field %d: got %q, want %q (Finding must carry only data fields, no severity/smell/advice)", i, got, want)
		}
	}
}
