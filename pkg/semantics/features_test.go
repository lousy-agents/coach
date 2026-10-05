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

// AC-5: multiple writes through the same parameter at distinct source
// locations must produce one finding per distinct location, and writing to
// the exact same expression location must never be double-counted.
func TestGoMutatesInput_DuplicateWritesDeduped(t *testing.T) {
	source := []byte(`package main

type Config struct {
	Name string
	Age  int
}

func f(cfg *Config) {
	cfg.Name = "x"
	cfg.Age = 1
	cfg.Name = "y"
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	got := countMutatesInputFindings(findings, "f:cfg")
	if got != 3 {
		t.Errorf("computeGoFeatures findings for %q: got %d mutates_input findings named %q (one per distinct mutation-expression location), want 3; findings=%+v", source, got, "f:cfg", findings)
	}

	seenLocations := map[uint]bool{}
	for _, f := range findings {
		if f.Kind != "mutates_input" || f.Name != "f:cfg" {
			continue
		}
		if seenLocations[f.Location.StartByte] {
			t.Errorf("computeGoFeatures findings for %q: duplicate mutates_input finding at the same Location.StartByte %d", source, f.Location.StartByte)
		}
		seenLocations[f.Location.StartByte] = true
	}
}
