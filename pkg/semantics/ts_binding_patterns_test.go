package semantics

import (
	"testing"
)

func TestTSMutatesInput_DestructuringAssignmentReadsDoNotCountAsTargets(t *testing.T) {
	tests := []tsMutatesInputSourceCase{
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
			expectNoFindingForDestructuringRead(t, tt)
		})
	}
}

func expectNoFindingForDestructuringRead(t *testing.T, tt tsMutatesInputSourceCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))
	for _, f := range findings {
		if f.Kind == "mutates_input" {
			t.Fatalf("computeTSFeatures for destructuring read %q: got mutates_input finding %+v, want none because p.x is read but not assigned", tt.source, f)
		}
	}
}

func TestTSMutatesInput_DestructuringAliasDoesNotShadowParameter(t *testing.T) {
	tests := []tsMutatesInputSourceCase{
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
			expectFindingDespiteDestructuringAlias(t, tt)
		})
	}
}

func expectFindingDespiteDestructuringAlias(t *testing.T, tt tsMutatesInputSourceCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))
	got := mustFindTSMutatesInput(t, tt.source, findings)

	if got.Name != "f:p" {
		t.Errorf("Finding.Name for %q = %q, want %q", tt.source, got.Name, "f:p")
	}
}
