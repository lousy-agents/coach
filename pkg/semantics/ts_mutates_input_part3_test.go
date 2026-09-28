package semantics

import (
	"testing"
)

// Story 2, delete: `delete p.x` on an identifier-bound parameter must
// yield a mutates_input Finding spanning the whole delete expression.
func TestTSMutatesInput_Delete(t *testing.T) {
	source := `function f(p) {
	delete p.x;
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
	if gotText != "delete p.x" {
		t.Errorf("Finding.Location text = %q, want %q", gotText, "delete p.x")
	}
	if got.Evidence != "delete p.x" {
		t.Errorf("Finding.Evidence = %q, want %q", got.Evidence, "delete p.x")
	}
}

func TestTSMutatesInput_ReboundParameterIsNotTrackedForLaterWrites(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
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
			body_tsMutatesInputPart3Test_81(t, tt)
		})
	}
}

// Review finding #3: a lexical binding declared inside a block shadows the
// function parameter. Mutating the local binding is not a mutation of the
// caller's input parameter and must not be attributed to f:p.
func TestTSMutatesInput_BlockLocalBindingShadowsParameter(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
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
			body_tsMutatesInputPart3Test_148(t, tt)
		})
	}
}

func TestTSMutatesInput_CompoundAssignment(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		wantName string
		evidence string
	}{
		{
			name: "property plus-equals",
			source: `function f(p) {
	p.x += 1;
}
`,
			wantName: "f:p",
			evidence: "p.x",
		},
		{
			name: "index logical-or assignment",
			source: `function f(p) {
	p["x"] ||= 1;
}
`,
			wantName: "f:p",
			evidence: `p["x"]`,
		},
		{
			name: "nested nullish assignment",
			source: `function f(p) {
	p.items[0].name ??= "x";
}
`,
			wantName: "f:p",
			evidence: "p.items[0].name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_tsMutatesInputPart3Test_200(t, tt)
		})
	}
}
