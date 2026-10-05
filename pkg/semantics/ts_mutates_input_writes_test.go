package semantics

import (
	"testing"
)

// Story 2/3, property assignment: `p.x = 1` inside a function body whose
// `p` is an identifier-bound parameter must yield one mutates_input
// Finding with the full Story 3 field shape.
func TestTSMutatesInput_PropertyAssignment(t *testing.T) {
	source := `function f(p) {
	p.x = 1;
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
	if gotText != "p.x" {
		t.Errorf("Finding.Location text = %q, want %q", gotText, "p.x")
	}
	if got.Confidence != "medium" {
		t.Errorf("Finding.Confidence = %q, want %q", got.Confidence, "medium")
	}
	if got.Evidence != "p.x" {
		t.Errorf("Finding.Evidence = %q, want %q", got.Evidence, "p.x")
	}
	if got.SuggestedSkill != "refactor-hidden-mutation" {
		t.Errorf("Finding.SuggestedSkill = %q, want %q", got.SuggestedSkill, "refactor-hidden-mutation")
	}
	if got.Recommendation == "" {
		t.Errorf("Finding.Recommendation is empty, want a non-empty sentence")
	}
}

// Story 2, index assignment: both `arr[0] = 1` (numeric index) and
// `obj['k'] = 1` (string index) on an identifier-bound parameter must each
// yield a mutates_input Finding.
func TestTSMutatesInput_IndexAssignment(t *testing.T) {
	tests := []tsIndexAssignmentCase{
		{
			name:       "numeric index",
			source:     "function f(arr) {\n\tarr[0] = 1;\n}\n",
			wantParam:  "arr",
			wantSuffix: "arr[0]",
		},
		{
			name:       "string index",
			source:     "function f(obj) {\n\tobj['k'] = 1;\n}\n",
			wantParam:  "obj",
			wantSuffix: "obj['k']",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectIndexAssignmentFinding(t, tt)
		})
	}
}

type tsIndexAssignmentCase struct {
	name       string
	source     string
	wantParam  string
	wantSuffix string
}

func expectIndexAssignmentFinding(t *testing.T, tt tsIndexAssignmentCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))
	got := mustFindTSMutatesInput(t, tt.source, findings)

	if got.Name != "f:"+tt.wantParam {
		t.Errorf("Finding.Name = %q, want %q", got.Name, "f:"+tt.wantParam)
	}
	gotText := tt.source[got.Location.StartByte:got.Location.EndByte]
	if gotText != tt.wantSuffix {
		t.Errorf("Finding.Location text = %q, want %q", gotText, tt.wantSuffix)
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
