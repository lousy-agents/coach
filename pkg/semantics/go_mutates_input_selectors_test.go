package semantics

import (
	"strings"
	"testing"
)

// Story 1/3: a selector write through a syntactically pointer-typed
// parameter (cfg.Name = "x", cfg *Config) must emit exactly one
// "mutates_input" Finding, with Name "<func>:<param>", Location pointing at
// the mutation expression (not the function declaration), and the coaching
// metadata fields set as specified.
func TestGoMutatesInput_PointerSelectorWrite(t *testing.T) {
	source := []byte(`package main

type Config struct {
	Name string
}

func f(cfg *Config) {
	cfg.Name = "x"
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	got := mutatesInputFinding(findings, "f:cfg")
	if got == nil {
		t.Fatalf("computeGoFeatures findings for %q: want a mutates_input finding named %q, got %+v", source, "f:cfg", findings)
	}
	if got.Evidence != "cfg.Name" {
		t.Errorf("mutates_input Evidence: got %q, want %q", got.Evidence, "cfg.Name")
	}
	if got.Confidence != "medium" {
		t.Errorf("mutates_input Confidence: got %q, want %q", got.Confidence, "medium")
	}
	if got.Recommendation == "" {
		t.Errorf("mutates_input Recommendation: got empty, want a non-empty recommendation")
	}
	if got.SuggestedSkill != "refactor-hidden-mutation" {
		t.Errorf("mutates_input SuggestedSkill: got %q, want %q", got.SuggestedSkill, "refactor-hidden-mutation")
	}
	wantExprStart := uint(strings.Index(string(source), `cfg.Name = "x"`))
	if got.Location.StartByte != wantExprStart {
		t.Errorf("mutates_input Location.StartByte: got %d, want %d (start of mutation expression, not the function declaration)", got.Location.StartByte, wantExprStart)
	}
}

// Copilot review fix: a nested selector write (cfg.Sub.Name = "x", where
// cfg is a pointer parameter) is still a caller-visible write through cfg
// one field deeper, and must resolve to the root identifier cfg rather than
// being silently missed because the selector's own operand is itself a
// selector_expression rather than a bare identifier.
func TestGoMutatesInput_NestedSelectorWrite(t *testing.T) {
	source := []byte(`package main

type Sub struct {
	Name string
}

type Config struct {
	Sub Sub
}

func f(cfg *Config) {
	cfg.Sub.Name = "x"
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	got := mutatesInputFinding(findings, "f:cfg")
	if got == nil {
		t.Fatalf("computeGoFeatures findings for nested selector write %q: want a mutates_input finding named %q, got %+v", source, "f:cfg", findings)
	}
	if got.Evidence != "cfg.Sub.Name" {
		t.Errorf("mutates_input Evidence: got %q, want %q", got.Evidence, "cfg.Sub.Name")
	}
}

// AC-4: a selector write on a plain (non-pointer/map/slice) value parameter
// mutates only a local copy, not caller-visible state, and must not emit a
// finding.
func TestGoMutatesInput_ValueParameterSelectorWriteNoFinding(t *testing.T) {
	source := []byte(`package main

type Config struct {
	Name string
}

func f(cfg Config) {
	cfg.Name = "x"
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if hasFinding(findings, "mutates_input", "f:cfg") {
		t.Errorf("computeGoFeatures findings for value-parameter selector write %q: want no mutates_input finding named %q, got %+v", source, "f:cfg", findings)
	}
}

// Copilot review fix: the same collapse also let a DIRECT selector write
// on a map/slice parameter pass the (bool) mutability check even though
// only pointer parameters are in scope for selector/dereference writes.
// This is not valid Go (a slice type has no fields) but this detector
// never type-checks the file, only its own syntax shape -- Tree-sitter
// parses "items.Foo = x" as a selector_expression assignment target
// regardless of whether Foo is a real field, so the fix must reject it at
// the syntax level rather than relying on the parameter merely being
// "mutable at all".
func TestGoMutatesInput_DirectSelectorWriteOnSliceParamNoFinding(t *testing.T) {
	source := []byte(`package main

func f(items []int) {
	items.Foo = "x"
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if hasFinding(findings, "mutates_input", "f:items") {
		t.Errorf("computeGoFeatures findings for direct selector write on slice parameter %q: want no mutates_input finding, got %+v", source, findings)
	}
}
