package semantics

import (
	"testing"
)

func TestTSMutatesInput_ControlFlowAndFunctionBindingsShadowParameter(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name: "for-of lexical binding",
			source: `function f(p, items) {
	for (const p of items) {
		p.x = 1;
	}
}
`,
		},
		{
			name: "for initializer lexical binding",
			source: `function f(p) {
	for (let p = {}; p; p = null) {
		p.x = 1;
	}
}
`,
		},
		{
			name: "function declaration binding",
			source: `function f(p) {
	function p() {}
	p.x = 1;
}
`,
		},
		{
			name: "destructured lexical binding",
			source: `function f(p, source) {
	const { p } = source;
	p.x = 1;
}
`,
		},
		{
			name: "var binding shadows for the rest of the function body",
			source: `function f(p) {
	{
		var p = {};
	}
	p.x = 1;
}
`,
		},
		{
			name: "class declaration binding",
			source: `function f(p) {
	{
		class p {}
		p.x = 1;
	}
}
`,
		},
		{
			name: "destructured catch binding",
			source: `function f(p) {
	try {
		throw {};
	} catch ({ p }) {
		p.x = 1;
	}
}
`,
		},
		{
			name: "switch case lexical binding",
			source: `function f(p, x) {
	switch (x) {
	case 1:
		let p = {};
		p.x = 1;
	}
}
`,
		},
		{
			name: "switch case lexical binding shadows sibling cases",
			source: `function f(p, x) {
	switch (x) {
	case 1:
		let p = {};
		break;
	case 2:
		p.x = 1;
	}
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_tsMutatesInputPart4Test_104(t, tt)
		})
	}
}

// D5, non-identifier parameters: destructured object/array patterns, rest
// patterns, and defaulted parameters are never tracked, so a body that
// looks like it mutates the pattern's inner bindings must not yield any
// mutates_input finding.
func TestTSMutatesInput_NonIdentifierParametersAreIgnored(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name:   "object pattern",
			source: "function f({x}) {\n\tx.y = 1;\n}\n",
		},
		{
			name:   "array pattern",
			source: "function f([a, b]) {\n\ta.y = 1;\n}\n",
		},
		{
			name:   "rest pattern",
			source: "function f(...rest) {\n\trest[0].y = 1;\n}\n",
		},
		{
			name:   "defaulted parameter",
			source: "function f(x = {}) {\n\tx.y = 1;\n}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_tsMutatesInputPart4Test_147(t, tt)
		})
	}
}

// Review finding #4: wrapper expressions around a parameter root do not
// change the write-through target. Parentheses and TypeScript non-null
// assertions must still resolve to the parameter being mutated.
func TestTSMutatesInput_WrappedParameterRoots(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		wantName string
		evidence string
	}{
		{
			name: "parenthesized property root",
			source: `function f(p) {
	(p).x = 1;
}
`,
			wantName: "f:p",
			evidence: "(p).x",
		},
		{
			name: "non-null assertion property root",
			source: `function g(p?: { x: number }) {
	p!.x = 1;
}
`,
			wantName: "g:p",
			evidence: "p!.x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_tsMutatesInputPart4Test_192(t, tt)
		})
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
