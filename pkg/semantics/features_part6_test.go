package semantics

import (
	"testing"
)

func TestGoMutatesInput_ParenthesizedDirectDereferenceAssignment(t *testing.T) {
	source := []byte(`package main

type Config struct {
	Name string
}

func f(cfg *Config) {
	(*cfg) = Config{}
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	got := mutatesInputFinding(findings, "f:cfg")
	if got == nil {
		t.Fatalf("computeGoFeatures findings for parenthesized dereference assignment %q: want a mutates_input finding named %q, got %+v", source, "f:cfg", findings)
	}
	if got.Evidence != "*cfg" {
		t.Errorf("mutates_input Evidence for parenthesized dereference assignment: got %q, want %q", got.Evidence, "*cfg")
	}
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

	_, findings := computeGoFeatures(root, source)

	if !hasFinding(findings, "pointer_return", "f") {
		t.Errorf("computeGoFeatures findings for pointer-returning function %q: want a pointer_return finding named %q, got %+v", source, "f", findings)
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

	_, findings := computeGoFeatures(root, source)

	if !hasFinding(findings, "pointer_return", "Method") {
		t.Errorf("computeGoFeatures findings for pointer-returning method %q: want a pointer_return finding named %q, got %+v", source, "Method", findings)
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

	_, findings := computeGoFeatures(root, source)

	if hasFinding(findings, "pointer_return", "f") {
		t.Errorf("computeGoFeatures findings for value-returning function %q: want no pointer_return finding named %q, got %+v", source, "f", findings)
	}
}

// Regression guard raised by review: AC-3.6 requires a finding when the
// result list contains at least one pointer_type anywhere in it, not just
// as the direct result node or a parameter_declaration's direct type field.
// A slice-of-pointer result (func f() []*T -> result: (slice_type element:
// (pointer_type ...))) previously produced no finding at all.
func TestFindings_PointerReturnOnSliceOfPointerResult(t *testing.T) {
	source := []byte(`package main

type T struct{}

func f() []*T {
	return nil
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if !hasFinding(findings, "pointer_return", "f") {
		t.Errorf("computeGoFeatures findings for []*T-returning function %q: want a pointer_return finding named %q, got %+v", source, "f", findings)
	}
}

// Regression guard: a map-value-of-pointer result among multiple named
// return values (func g() (map[string]*T, error) -> the first
// parameter_declaration's type field is map_type, not pointer_type
// directly) previously produced no finding.
func TestFindings_PointerReturnOnMapValuePointerAmongMultipleResults(t *testing.T) {
	source := []byte(`package main

type T struct{}

func g() (map[string]*T, error) {
	return nil, nil
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if !hasFinding(findings, "pointer_return", "g") {
		t.Errorf("computeGoFeatures findings for map[string]*T-returning function %q: want a pointer_return finding named %q, got %+v", source, "g", findings)
	}
}

// Regression guard: a channel-of-pointer result (func h() chan *T ->
// result: (channel_type value: (pointer_type ...))) previously produced no
// finding.
func TestFindings_PointerReturnOnChannelOfPointerResult(t *testing.T) {
	source := []byte(`package main

type T struct{}

func h() chan *T {
	return nil
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if !hasFinding(findings, "pointer_return", "h") {
		t.Errorf("computeGoFeatures findings for chan *T-returning function %q: want a pointer_return finding named %q, got %+v", source, "h", findings)
	}
}

// Regression guard: the same composite-result rule must apply to methods,
// not just functions, since checkPointerReturn is shared between them.
func TestFindings_PointerReturnOnMethodWithSliceOfPointerResult(t *testing.T) {
	source := []byte(`package main

type T struct{}
type Recv struct{}

func (r *Recv) Items() []*T {
	return nil
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if !hasFinding(findings, "pointer_return", "Items") {
		t.Errorf("computeGoFeatures findings for []*T-returning method %q: want a pointer_return finding named %q, got %+v", source, "Items", findings)
	}
}

// Story 1: a dereference write through a pointer-typed parameter
// ((*cfg).Name = "x") must also emit a mutates_input finding.
func TestGoMutatesInput_DereferenceWrite(t *testing.T) {
	source := []byte(`package main

type Config struct {
	Name string
}

func f(cfg *Config) {
	(*cfg).Name = "x"
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if !hasFinding(findings, "mutates_input", "f:cfg") {
		t.Errorf("computeGoFeatures findings for dereference write %q: want a mutates_input finding named %q, got %+v", source, "f:cfg", findings)
	}
}

// Story 1: an index write on a map-typed parameter (values[key] = x,
// values map[string]int) must emit a mutates_input finding.
func TestGoMutatesInput_MapIndexWrite(t *testing.T) {
	source := []byte(`package main

func f(values map[string]int) {
	values["k"] = 1
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if !hasFinding(findings, "mutates_input", "f:values") {
		t.Errorf("computeGoFeatures findings for map index write %q: want a mutates_input finding named %q, got %+v", source, "f:values", findings)
	}
}
