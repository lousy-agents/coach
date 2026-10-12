package semantics

import (
	"fmt"
	"testing"
)

// Story 3, anonymous naming: an anonymous function expression whose
// identifier-bound parameter is mutated must use "anonymous@<start_byte>"
// for the Name's function half, not borrow a name from an enclosing
// variable_declarator.
func TestTSMutatesInput_AnonymousFunctionExpressionUsesStartByteName(t *testing.T) {
	source := `const f = function (p) {
	p.x = 1;
};
`
	root, closeTree := mustParseTS(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))
	got := mustFindTSMutatesInput(t, source, findings)

	funcStart := len("const f = ")
	wantName := fmt.Sprintf("anonymous@%d:p", funcStart)
	if got.Name != wantName {
		t.Errorf("Finding.Name = %q, want %q", got.Name, wantName)
	}
}

// Story 3, anonymous naming (arrow variant): an arrow function assigned to
// a variable, whose identifier-bound parameter is mutated, must also use
// "anonymous@<start_byte>", since arrow functions never have a syntactic
// name field of their own.
func TestTSMutatesInput_AnonymousArrowFunctionUsesStartByteName(t *testing.T) {
	source := `const f = (p) => {
	p.x = 1;
};
`
	root, closeTree := mustParseTS(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))
	got := mustFindTSMutatesInput(t, source, findings)

	funcStart := len("const f = ")
	wantName := fmt.Sprintf("anonymous@%d:p", funcStart)
	if got.Name != wantName {
		t.Errorf("Finding.Name = %q, want %q", got.Name, wantName)
	}
}

// Story 2/AC4: reassigning the parameter binding itself (`p = other`) is a
// rebind, not a write-through, and must not yield a mutates_input Finding.
func TestTSMutatesInput_ParameterRebindingIsNotFlagged(t *testing.T) {
	source := `function f(p) {
	p = other;
}
`
	root, closeTree := mustParseTS(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))
	for _, f := range findings {
		if f.Kind == "mutates_input" {
			t.Fatalf("computeTSFeatures for %q: got mutates_input finding %+v, want none (plain rebind is not a write-through)", source, f)
		}
	}
}

type tsMutatesInputSourceCase struct {
	name   string
	source string
}

func TestTSMutatesInput_ReboundParameterIsNotTrackedForLaterWrites(t *testing.T) {
	tests := []tsMutatesInputSourceCase{
		{
			name: "assignment rebinding",
			source: `function f(p) {
	p = {};
	p.x = 1;
}
`,
		},
		{
			name: "for-of rebinding",
			source: `function f(p, items) {
	for (p of items) {
		p.x = 1;
	}
}
`,
		},
		{
			name: "augmented assignment rebinding",
			source: `function f(p) {
	p += other;
	p.x = 1;
}
`,
		},
		{
			name: "object destructuring rebinding",
			source: `function f(p, source) {
	({ p } = source);
	p.x = 1;
}
`,
		},
		{
			name: "array destructuring rebinding",
			source: `function f(p, source) {
	[p] = source;
	p.x = 1;
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectNoFindingAfterParameterRebind(t, tt)
		})
	}
}

func expectNoFindingAfterParameterRebind(t *testing.T, tt tsMutatesInputSourceCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))

	for _, f := range findings {
		if f.Kind == "mutates_input" {
			t.Fatalf("computeTSFeatures for %q: got mutates_input finding %+v, want none after parameter rebinding", tt.source, f)
		}
	}
}

func TestTSMutatesInput_VarParameterRebindDoesNotSuppressEarlierMutation(t *testing.T) {
	source := `function f(p) {
	p.x = 1;
	var p = {};
}
`
	root, closeTree := mustParseTS(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))
	got := mutatesInputFindingNamed(findings, "f:p", "p.x")
	if got == nil {
		t.Fatalf("computeTSFeatures for %q: want mutates_input before same-name var rebind, got %+v", source, findings)
	}
}

func TestTSMutatesInput_NoInitializerVarParameterDoesNotSuppressLaterMutation(t *testing.T) {
	source := `function f(p) {
	var p;
	p.x = 1;
}
`
	root, closeTree := mustParseTS(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))
	got := mutatesInputFindingNamed(findings, "f:p", "p.x")
	if got == nil {
		t.Fatalf("computeTSFeatures for %q: want mutates_input after no-initializer var parameter declaration, got %+v", source, findings)
	}
}
