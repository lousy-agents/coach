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

type tsMutatesInputFindingCase struct {
	name     string
	source   string
	wantName string
	evidence string
}

func TestTSMutatesInput_CompoundAssignment(t *testing.T) {
	tests := []tsMutatesInputFindingCase{
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
			expectCompoundAssignmentFinding(t, tt)
		})
	}
}

func expectCompoundAssignmentFinding(t *testing.T, tt tsMutatesInputFindingCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))

	got := mutatesInputFindingNamed(findings, tt.wantName, tt.evidence)
	if got == nil {
		t.Fatalf("computeTSFeatures for compound assignment %q: want mutates_input named %q with evidence %q, got %+v", tt.source, tt.wantName, tt.evidence, findings)
	}
}

func TestTSMutatesInput_UpdateExpressionsMutateParameterRoots(t *testing.T) {
	tests := []tsMutatesInputFindingCase{
		{
			name: "postfix property increment",
			source: `function f(p) {
	p.x++;
}
`,
			wantName: "f:p",
			evidence: "p.x",
		},
		{
			name: "prefix index decrement",
			source: `function f(arr) {
	--arr[0];
}
`,
			wantName: "f:arr",
			evidence: "arr[0]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectTSUpdateExpressionFinding(t, tt)
		})
	}
}

func expectTSUpdateExpressionFinding(t *testing.T, tt tsMutatesInputFindingCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))

	got := mutatesInputFindingNamed(findings, tt.wantName, tt.evidence)
	if got == nil {
		t.Fatalf("computeTSFeatures for update expression %q: want a mutates_input finding named %q with evidence %q, got %+v", tt.source, tt.wantName, tt.evidence, findings)
	}
}

func TestTSMutatesInput_DestructuringAssignmentTargets(t *testing.T) {
	tests := []tsMutatesInputFindingCase{
		{
			name: "object pattern property target",
			source: `function f(p, src) {
	({ x: p.x } = src);
}
`,
			wantName: "f:p",
			evidence: "p.x",
		},
		{
			name: "array pattern index target",
			source: `function f(p, src) {
	[p[0]] = src;
}
`,
			wantName: "f:p",
			evidence: "p[0]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectDestructuringAssignmentFinding(t, tt)
		})
	}
}

func expectDestructuringAssignmentFinding(t *testing.T, tt tsMutatesInputFindingCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))
	got := mutatesInputFindingNamed(findings, tt.wantName, tt.evidence)
	if got == nil {
		t.Fatalf("computeTSFeatures for destructuring assignment %q: want mutates_input named %q with evidence %q, got %+v", tt.source, tt.wantName, tt.evidence, findings)
	}
}
