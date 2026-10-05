package semantics

import (
	"testing"
)

// Review finding #3: a lexical binding declared inside a block shadows the
// function parameter. Mutating the local binding is not a mutation of the
// caller's input parameter and must not be attributed to f:p.
func TestTSMutatesInput_BlockLocalBindingShadowsParameter(t *testing.T) {
	tests := []tsMutatesInputSourceCase{
		{
			name: "let binding",
			source: `function f(p) {
	{
		let p = {};
		p.x = 1;
	}
}
`,
		},
		{
			name: "const binding",
			source: `function f(p) {
	{
		const p = {};
		p.x = 1;
	}
}
`,
		},
		{
			name: "var binding",
			source: `function f(p) {
	{
		var p = {};
		p.x = 1;
	}
}
`,
		},
		{
			name: "catch binding",
			source: `function f(p) {
	try {
		throw new Error();
	} catch (p) {
		p.x = 1;
	}
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectNoFindingForBlockLocalBinding(t, tt)
		})
	}
}

func expectNoFindingForBlockLocalBinding(t *testing.T, tt tsMutatesInputSourceCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))

	for _, f := range findings {
		if f.Kind == "mutates_input" {
			t.Fatalf("computeTSFeatures for %q: got mutates_input finding %+v, want none because the write targets a block-local binding", tt.source, f)
		}
	}
}

func TestTSMutatesInput_ControlFlowAndFunctionBindingsShadowParameter(t *testing.T) {
	tests := []tsMutatesInputSourceCase{
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
			expectNoFindingForShadowingBinding(t, tt)
		})
	}
}

func expectNoFindingForShadowingBinding(t *testing.T, tt tsMutatesInputSourceCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))

	for _, f := range findings {
		if f.Kind == "mutates_input" {
			t.Fatalf("computeTSFeatures for %q: got mutates_input finding %+v, want none because the write targets a binding that shadows the parameter", tt.source, f)
		}
	}
}

func TestTSMutatesInput_PredeclaredLocalBindingsShadowParameterForWholeScope(t *testing.T) {
	tests := []tsMutatesInputSourceCase{
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
			expectNoFindingForPredeclaredLocal(t, tt)
		})
	}
}

func expectNoFindingForPredeclaredLocal(t *testing.T, tt tsMutatesInputSourceCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))

	for _, f := range findings {
		if f.Kind == "mutates_input" {
			t.Fatalf("computeTSFeatures for predeclared local shadowing %q: got mutates_input finding %+v, want none because the write targets a local binding, not the parameter", tt.source, f)
		}
	}
}

func TestTSMutatesInput_InnerVarBindingShadowsOuterParameterBeforeDeclaration(t *testing.T) {
	source := `function outer(p) {
	function inner() {
		p.x = 1;
		var p = {};
	}
}
`
	root, closeTree := mustParseTS(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))
	for _, f := range findings {
		if f.Kind == "mutates_input" {
			t.Fatalf("computeTSFeatures for %q: got mutates_input finding %+v, want none because inner var shadows the outer parameter", source, f)
		}
	}
}
