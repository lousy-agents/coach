package semantics

import (
	"testing"
)

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

// Review finding #1: assigning directly through a pointer-typed parameter's
// dereference (*cfg = Config{...}) mutates the caller-visible value and must
// emit mutates_input, even though the assignment target is not a selector.
func TestGoMutatesInput_DirectPointerDereferenceAssignment(t *testing.T) {
	source := []byte(`package main

type Config struct {
	Name string
}

func f(cfg *Config) {
	*cfg = Config{Name: "x"}
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	got := mutatesInputFinding(findings, "f:cfg")
	if got == nil {
		t.Fatalf("computeGoFeatures findings for direct pointer dereference assignment %q: want a mutates_input finding named %q, got %+v", source, "f:cfg", findings)
	}
	if got.Evidence != "*cfg" {
		t.Errorf("mutates_input Evidence: got %q, want %q", got.Evidence, "*cfg")
	}
}

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

// Review finding #4: ordinary parentheses around the parameter root do not
// change the write-through target. Selector and index writes rooted at
// parenthesized pointer/map/slice parameters must still be detected.
func TestGoMutatesInput_ParenthesizedRootWrites(t *testing.T) {
	tests := []goMutatesInputFindingCase{
		{
			name: "selector on parenthesized pointer parameter",
			source: `package main

type Config struct {
	Name string
}

func f(cfg *Config) {
	(cfg).Name = "x"
}
`,
			wantName: "f:cfg",
			evidence: "(cfg).Name",
		},
		{
			name: "index on parenthesized slice parameter",
			source: `package main

func g(items []int) {
	(items)[0] = 1
}
`,
			wantName: "g:items",
			evidence: "(items)[0]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectParenthesizedRootWriteFinding(t, tt)
		})
	}
}

type goMutatesInputFindingCase struct {
	name     string
	source   string
	wantName string
	evidence string
}

func expectParenthesizedRootWriteFinding(t *testing.T, tt goMutatesInputFindingCase) {
	root, closeTree := mustParseGo(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeGoFeatures(root, []byte(tt.source))

	got := mutatesInputFinding(findings, tt.wantName)
	if got == nil {
		t.Fatalf("computeGoFeatures findings for %q: want a mutates_input finding named %q, got %+v", tt.source, tt.wantName, findings)
	}
	if got.Evidence != tt.evidence {
		t.Errorf("mutates_input Evidence: got %q, want %q", got.Evidence, tt.evidence)
	}
}
