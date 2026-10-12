package semantics

import (
	"testing"
)

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

// Story 1: an index write on a slice-typed parameter (items[i] = x,
// items []int) must emit a mutates_input finding.
func TestGoMutatesInput_SliceIndexWrite(t *testing.T) {
	source := []byte(`package main

func f(items []int) {
	items[0] = 2
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if !hasFinding(findings, "mutates_input", "f:items") {
		t.Errorf("computeGoFeatures findings for slice index write %q: want a mutates_input finding named %q, got %+v", source, "f:items", findings)
	}
}

// Copilot review fix: an index_expression target's operand can itself be a
// nested selector/index chain (cfg.Items[0], not just a bare identifier
// like items[0]), and must resolve to its root identifier the same way
// selector_expression targets already do, so a caller-visible index write
// reached through a pointer-typed parameter's field is still detected.
func TestGoMutatesInput_NestedIndexWrite(t *testing.T) {
	source := []byte(`package main

type Config struct {
	Items []int
}

func f(cfg *Config) {
	cfg.Items[0] = 1
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	got := mutatesInputFinding(findings, "f:cfg")
	if got == nil {
		t.Fatalf("computeGoFeatures findings for nested index write %q: want a mutates_input finding named %q, got %+v", source, "f:cfg", findings)
	}
	if got.Evidence != "cfg.Items[0]" {
		t.Errorf("mutates_input Evidence: got %q, want %q", got.Evidence, "cfg.Items[0]")
	}
}

// Copilot review fix: mutableParamTypes previously collapsed pointer/map/
// slice parameters into a single bool, so a DIRECT index write on a
// pointer parameter (cfg[0] = x, valid Go only for a pointer-to-array) was
// treated the same as a direct index write on a map/slice parameter, even
// though the acceptance criteria scope index writes to map/slice
// parameters specifically. A direct index target's root kind must now
// match exactly.
func TestGoMutatesInput_DirectIndexWriteOnPointerParamNoFinding(t *testing.T) {
	source := []byte(`package main

func f(cfg *[5]int) {
	cfg[0] = 1
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if hasFinding(findings, "mutates_input", "f:cfg") {
		t.Errorf("computeGoFeatures findings for direct index write on pointer parameter %q: want no mutates_input finding, got %+v", source, findings)
	}
}
