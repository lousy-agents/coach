package semantics

import (
	"testing"
)

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

type tsMethodCallCountCase struct {
	name      string
	source    string
	wantCount int
}

func expectMutatingMethodCallFindingCount(t *testing.T, tt tsMethodCallCountCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))
	var got []Finding
	for _, f := range findings {
		if f.Kind == "mutates_input" {
			got = append(got, f)
		}
	}
	if len(got) != tt.wantCount {
		t.Fatalf("computeTSFeatures for %q: got %d mutates_input findings (%+v), want %d", tt.source, len(got), findings, tt.wantCount)
	}
}

// Copilot review fix: a mutating method call on a nested receiver
// (p.items.push(1), where p is an identifier-bound parameter) is still a
// caller-visible mutation rooted at p, and must be detected even though
// the call's receiver object is itself a member_expression rather than a
// bare identifier.
func TestTSMutatesInput_MutatingMethodCallOnNestedReceiver(t *testing.T) {
	source := `function f(p) {
	p.items.push(1);
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
	if gotText != "p.items.push" {
		t.Errorf("Finding.Location text = %q, want %q", gotText, "p.items.push")
	}
}

func TestTSMutatesInput_BracketNotationMutatingMethodCall(t *testing.T) {
	source := `function f(arr) {
	arr["push"](1);
}
`
	root, closeTree := mustParseTS(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))

	got := mutatesInputFindingNamed(findings, "f:arr", `arr["push"]`)
	if got == nil {
		t.Fatalf("computeTSFeatures for bracket-notation mutating method call %q: want a mutates_input finding named %q with evidence %q, got %+v", source, "f:arr", `arr["push"]`, findings)
	}
}

// Copilot review fix: Evidence/Location for both an assignment and a
// mutating method call must stay bounded to the mutated target/receiver,
// not grow with an arbitrarily long right-hand side or argument list, so
// evidence stays short and consistent with the Go detector's target-only
// evidence.
func TestTSMutatesInput_EvidenceExcludesLongRHSAndArguments(t *testing.T) {
	source := `function f(p) {
	p.x = someVeryLargeExpressionThatShouldNotAppearInEvidence();
	p.items.push(anotherVeryLargeExpressionThatShouldNotAppearInEvidence());
}
`
	root, closeTree := mustParseTS(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))

	assignment := mutatesInputFindingNamed(findings, "f:p", "p.x")
	if assignment == nil {
		t.Fatalf("computeTSFeatures for %q: want a mutates_input finding with Evidence %q, got %+v", source, "p.x", findings)
	}

	call := mutatesInputFindingNamed(findings, "f:p", "p.items.push")
	if call == nil {
		t.Fatalf("computeTSFeatures for %q: want a mutates_input finding with Evidence %q, got %+v", source, "p.items.push", findings)
	}
}
