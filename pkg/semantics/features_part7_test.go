package semantics

import (
	"testing"
)

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

// AC-3: reassigning the parameter variable itself (cfg = other) rebinds the
// local variable rather than writing through it, and must not emit a
// finding, even though cfg's declared type is a pointer.
func TestGoMutatesInput_PlainParameterReassignmentNoFinding(t *testing.T) {
	source := []byte(`package main

type Config struct{}

func f(cfg *Config, other *Config) {
	cfg = other
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if hasFinding(findings, "mutates_input", "f:cfg") {
		t.Errorf("computeGoFeatures findings for plain parameter reassignment %q: want no mutates_input finding named %q, got %+v", source, "f:cfg", findings)
	}
}

func TestGoMutatesInput_ReboundParameterIsNotTrackedForLaterWrites(t *testing.T) {
	source := []byte(`package main

type Config struct {
	Name string
}

func f(cfg *Config) {
	cfg = &Config{}
	cfg.Name = "local"
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if hasFinding(findings, "mutates_input", "f:cfg") {
		t.Errorf("computeGoFeatures findings for rebound parameter %q: want no mutates_input finding named %q, got %+v", source, "f:cfg", findings)
	}
}

// Review finding #2: a block-local binding introduced with := shadows an
// outer mutable parameter. Mutating that local binding is not a mutation of
// the caller's input parameter and must not be attributed to f:cfg.
func TestGoMutatesInput_BlockLocalShortVarShadowsOuterParameter(t *testing.T) {
	source := []byte(`package main

type Config struct {
	Name string
}

func f(cfg *Config) {
	{
		cfg := &Config{}
		cfg.Name = "local"
	}
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if hasFinding(findings, "mutates_input", "f:cfg") {
		t.Errorf("computeGoFeatures findings for block-local shadowed parameter %q: want no mutates_input finding named %q, got %+v", source, "f:cfg", findings)
	}
}

// TestGoMutatesInput_TypeSwitchAliasDoesNotShadowAfterSwitch guards against a
// regression where a type-switch alias's shadowing leaked past the end of
// the switch statement, causing later mutations of the outer parameter in
// the same enclosing block to go undetected.
func TestGoMutatesInput_TypeSwitchAliasDoesNotShadowAfterSwitch(t *testing.T) {
	source := []byte(`package main

type Config struct {
	Name string
}

func f(cfg *Config, value any) {
	switch cfg := value.(type) {
	case *Config:
		cfg.Name = "local"
	}
	cfg.Name = "mutated"
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if !hasFinding(findings, "mutates_input", "f:cfg") {
		t.Errorf("computeGoFeatures findings for %q: want mutates_input finding for %q (mutation after type switch refers to outer parameter), got %+v", source, "f:cfg", findings)
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

// Copilot review fix: derefOperand previously unwrapped any
// unary_expression inside parentheses without checking its operator, so a
// parenthesized non-dereference unary expression like (&cfg) could be
// mis-resolved as if it were (*cfg) and misattribute the write to cfg
// (Tree-sitter parses this syntactically even though it would not
// type-check: &cfg is **Config, which does not have a Name field the way
// *Config's (*cfg) does -- this detector never type-checks the file, only
// its own syntax shape, so the fix must reject this at the syntax level).
func TestGoMutatesInput_ParenthesizedAddressOfIsNotADereference(t *testing.T) {
	source := []byte(`package main

type Config struct {
	Name string
}

func f(cfg *Config) {
	(&cfg).Name = "y"
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if hasFinding(findings, "mutates_input", "f:cfg") {
		t.Errorf("computeGoFeatures findings for parenthesized address-of (not a dereference) %q: want no mutates_input finding, got %+v", source, findings)
	}
}
