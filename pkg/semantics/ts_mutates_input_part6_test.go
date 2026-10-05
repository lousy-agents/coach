package semantics

import (
	"fmt"
	"testing"
)

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

// Story 3, anonymous naming (arrow variant): an arrow function assigned to
// a variable, whose identifier-bound parameter is mutated, must also use
// "anonymous@<start_byte>", since arrow functions never have a syntactic
// name field of their own.
func TestTSMutatesInput_AnonymousArrowFunctionUsesStartByteName(t *testing.T) {
	source := `const f = (p) => {
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
