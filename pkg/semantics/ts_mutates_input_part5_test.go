package semantics

import (
	"fmt"
	"testing"
)

func TestTSMutatesInput_PredeclaredLocalBindingsShadowParameterForWholeScope(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name: "block lexical binding shadows before declaration",
			source: `function f(p) {
	{
		p.x = 1;
		let p = {};
	}
}
`,
		},
		{
			name: "function declaration shadows before declaration",
			source: `function f(p) {
	p.x = 1;
	function p() {}
}
`,
		},
		{
			name: "same-name var does not cancel function declaration shadow",
			source: `function f(p) {
	p.x = 1;
	function p() {}
	var p = {};
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_tsMutatesInputPart5Test_43(t, tt)
		})
	}
}

func TestTSMutatesInput_DestructuringAssignmentReadsDoNotCountAsTargets(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name: "computed object key reads parameter property",
			source: `function f(p, src) {
	({ [p.x]: local } = src);
}
`,
		},
		{
			name: "default initializer reads parameter property",
			source: `function f(p, src) {
	({ x: local = p.x } = src);
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_tsMutatesInputPart5Test_80(t, tt)
		})
	}
}

func TestTSMutatesInput_DestructuringAliasDoesNotShadowParameter(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name: "object pattern alias",
			source: `function f(p, source) {
	const { p: q } = source;
	p.x = 1;
}
`,
		},
		{
			name: "catch object pattern alias",
			source: `function f(p) {
	try {
		throw {};
	} catch ({ p: q }) {
		p.x = 1;
	}
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_tsMutatesInputPart5Test_121(t, tt)
		})
	}
}

// Copilot review fix: a nested member write (p.x.y = 1, where p is an
// identifier-bound parameter) is still a caller-visible write through p one
// property deeper, and must resolve to the root identifier p rather than
// being missed because the assignment's own object is itself a
// member_expression rather than a bare identifier.
func TestTSMutatesInput_NestedPropertyAssignment(t *testing.T) {
	source := `function f(p) {
	p.x.y = 1;
}
`
	root, closeTree := mustParseTS(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))
	got := mustFindTSMutatesInput(t, source, findings)

	if got.Name != "f:p" {
		t.Errorf("Finding.Name = %q, want %q", got.Name, "f:p")
	}
	gotText := source[got.Location.StartByte:got.Location.EndByte]
	if gotText != "p.x.y" {
		t.Errorf("Finding.Location text = %q, want %q", gotText, "p.x.y")
	}
}

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
