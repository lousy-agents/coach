package semantics

import (
	"testing"
)

// A pointer-typed receiver mutated through a selector is not a declared
// parameter (the issue explicitly scopes this feature to parameters, not
// receivers), so a method mutating only its receiver must not emit a
// mutates_input finding.
func TestGoMutatesInput_ReceiverMutationIsNotAParameter(t *testing.T) {
	source := []byte(`package main

type T struct {
	Name string
}

func (t *T) Method() {
	t.Name = "x"
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	for _, f := range findings {
		if f.Kind == "mutates_input" {
			t.Errorf("computeGoFeatures findings for receiver-only mutation %q: want no mutates_input findings (receiver is not a parameter), got %+v", source, findings)
		}
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

// Copilot review fix: a func_literal that declares its own parameter with
// the same name as an outer function's mutable parameter (both named cfg,
// both *Config) introduces a distinct binding per normal Go scoping. The
// closure's own mutation of its cfg must not be misattributed to the outer
// function f's cfg.
func TestGoMutatesInput_FuncLiteralShadowsOuterParameter(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name: "ordinary parameter",
			source: `package main

type Config struct {
	Name string
}

func f(cfg *Config) {
	g := func(cfg *Config) {
		cfg.Name = "shadowed"
	}
	g(cfg)
}
`,
		},
		{
			name: "variadic parameter",
			source: `package main

type Config struct {
	Name string
}

func f(cfg *Config) {
	g := func(cfg ...*Config) {
		cfg[0].Name = "shadowed"
	}
	g(cfg)
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_featuresPart5Test_113(t, tt)
		})
	}
}

func TestGoMutatesInput_ControlFlowInitializerBindingsShadowOuterParameter(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name: "range loop variable shadows outer parameter",
			source: `package main

type Config struct {
	Name string
}

func f(cfg *Config, configs []*Config) {
	for _, cfg := range configs {
		cfg.Name = "local"
	}
}
`,
		},
		{
			name: "if initializer shadows outer parameter",
			source: `package main

type Config struct {
	Name string
}

func f(cfg *Config) {
	if cfg := (&Config{}); cfg != nil {
		cfg.Name = "local"
	}
}
`,
		},
		{
			name: "for initializer shadows outer parameter",
			source: `package main

type Config struct {
	Name string
}

func f(cfg *Config) {
	for cfg := (&Config{}); cfg != nil; {
		cfg.Name = "local"
		break
	}
}
`,
		},
		{
			name: "switch initializer shadows outer parameter",
			source: `package main

type Config struct {
	Name string
}

func f(cfg *Config) {
	switch cfg := (&Config{}); {
	case cfg != nil:
		cfg.Name = "local"
	}
}
`,
		},
		{
			name: "type switch guard shadows outer parameter",
			source: `package main

type Config struct {
	Name string
}

func f(cfg *Config, value any) {
	switch cfg := value.(type) {
	case *Config:
		cfg.Name = "local"
	}
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_featuresPart5Test_213(t, tt)
		})
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
