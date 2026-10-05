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

// Story 2, known mutating method calls: each representative method
// (push, sort, set, delete-as-a-method) called on an identifier-bound
// parameter must yield a mutates_input Finding; an arbitrary custom method
// (setName) must not.
func TestTSMutatesInput_MutatingMethodCalls(t *testing.T) {
	tests := []tsMethodCallCountCase{
		{name: "copyWithin", source: "function f(arr) {\n\tarr.copyWithin(0, 1);\n}\n", wantCount: 1},
		{name: "fill", source: "function f(arr) {\n\tarr.fill(1);\n}\n", wantCount: 1},
		{name: "pop", source: "function f(arr) {\n\tarr.pop();\n}\n", wantCount: 1},
		{name: "push", source: "function f(arr) {\n\tarr.push(1);\n}\n", wantCount: 1},
		{name: "reverse", source: "function f(arr) {\n\tarr.reverse();\n}\n", wantCount: 1},
		{name: "shift", source: "function f(arr) {\n\tarr.shift();\n}\n", wantCount: 1},
		{name: "sort", source: "function f(arr) {\n\tarr.sort();\n}\n", wantCount: 1},
		{name: "splice", source: "function f(arr) {\n\tarr.splice(0, 1);\n}\n", wantCount: 1},
		{name: "unshift", source: "function f(arr) {\n\tarr.unshift(1);\n}\n", wantCount: 1},
		{name: "set", source: "function f(m) {\n\tm.set('k', 1);\n}\n", wantCount: 1},
		{name: "add", source: "function f(s) {\n\ts.add(1);\n}\n", wantCount: 1},
		{name: "delete method", source: "function f(m) {\n\tm.delete('k');\n}\n", wantCount: 1},
		{name: "clear", source: "function f(m) {\n\tm.clear();\n}\n", wantCount: 1},
		{name: "custom method excluded", source: "function f(user) {\n\tuser.setName('x');\n}\n", wantCount: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectMutatingMethodCallFindingCount(t, tt)
		})
	}
}

// AC6, nested attribution: a nested arrow function that mutates the outer
// function's parameter (without itself declaring a same-named parameter)
// must have the finding attributed to the OUTER function's name.
func TestTSMutatesInput_NestedFunctionAttributesToOuterOwner(t *testing.T) {
	source := `function outer(p) {
	const helper = () => {
		p.x = 1;
	};
	helper();
}
`
	root, closeTree := mustParseTS(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))
	got := mustFindTSMutatesInput(t, source, findings)

	if got.Name != "outer:p" {
		t.Errorf("Finding.Name = %q, want %q (nested arrow's mutation of outer's parameter attributes to outer)", got.Name, "outer:p")
	}
}
