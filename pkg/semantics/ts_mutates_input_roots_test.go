package semantics

import (
	"testing"
)

// Review finding #4: wrapper expressions around a parameter root do not
// change the write-through target. Parentheses and TypeScript non-null
// assertions must still resolve to the parameter being mutated.
func TestTSMutatesInput_WrappedParameterRoots(t *testing.T) {
	tests := []tsMutatesInputFindingCase{
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
			expectWrappedRootFinding(t, tt)
		})
	}
}

func expectWrappedRootFinding(t *testing.T, tt tsMutatesInputFindingCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))
	got := mustFindTSMutatesInput(t, tt.source, findings)

	if got.Name != tt.wantName {
		t.Errorf("Finding.Name = %q, want %q", got.Name, tt.wantName)
	}
	gotText := tt.source[got.Location.StartByte:got.Location.EndByte]
	if gotText != tt.evidence {
		t.Errorf("Finding.Location text = %q, want %q", gotText, tt.evidence)
	}
}

// D5, non-identifier parameters: destructured object/array patterns, rest
// patterns, and defaulted parameters are never tracked, so a body that
// looks like it mutates the pattern's inner bindings must not yield any
// mutates_input finding.
func TestTSMutatesInput_NonIdentifierParametersAreIgnored(t *testing.T) {
	tests := []tsMutatesInputSourceCase{
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
			expectNoFindingForNonIdentifierParameter(t, tt)
		})
	}
}

func expectNoFindingForNonIdentifierParameter(t *testing.T, tt tsMutatesInputSourceCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))
	for _, f := range findings {
		if f.Kind == "mutates_input" {
			t.Fatalf("computeTSFeatures for %q: got mutates_input finding %+v, want none (non-identifier-bound parameter must never be tracked)", tt.source, f)
		}
	}
}
