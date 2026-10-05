package semantics

import (
	"testing"
)

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

// Copilot review fix: a func_literal that declares its own parameter with
// the same name as an outer function's mutable parameter (both named cfg,
// both *Config) introduces a distinct binding per normal Go scoping. The
// closure's own mutation of its cfg must not be misattributed to the outer
// function f's cfg.
func TestGoMutatesInput_FuncLiteralShadowsOuterParameter(t *testing.T) {
	tests := []goShadowedParameterCase{
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
			expectClosureShadowedParameterIgnored(t, tt)
		})
	}
}

type goShadowedParameterCase struct {
	name   string
	source string
}

func expectClosureShadowedParameterIgnored(t *testing.T, tt goShadowedParameterCase) {
	source := []byte(tt.source)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if hasFinding(findings, "mutates_input", "f:cfg") {
		t.Errorf("computeGoFeatures findings for closure-shadowed parameter %q: want no mutates_input finding named %q (closure's cfg is a distinct binding), got %+v", source, "f:cfg", findings)
	}
}

func TestGoMutatesInput_ControlFlowInitializerBindingsShadowOuterParameter(t *testing.T) {
	tests := []goShadowedParameterCase{
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
			expectShadowedParameterIgnored(t, tt)
		})
	}
}

func expectShadowedParameterIgnored(t *testing.T, tt goShadowedParameterCase) {
	source := []byte(tt.source)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	if hasFinding(findings, "mutates_input", "f:cfg") {
		t.Errorf("computeGoFeatures findings for %q: want no mutates_input finding for shadowed parameter %q, got %+v", tt.source, "f:cfg", findings)
	}
}
