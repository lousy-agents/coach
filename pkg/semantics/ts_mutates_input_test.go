package semantics

import (
	"fmt"
	"testing"
)

// mustFindTSMutatesInput asserts findings contains exactly one
// "mutates_input" Finding and returns it.
func mustFindTSMutatesInput(t *testing.T, source string, findings []Finding) Finding {
	t.Helper()
	var got []Finding
	for _, f := range findings {
		if f.Kind == "mutates_input" {
			got = append(got, f)
		}
	}
	if len(got) != 1 {
		t.Fatalf("computeTSFeatures for %q: got %d mutates_input findings (%+v), want exactly 1", source, len(got), findings)
	}
	return got[0]
}

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
	tests := []struct {
		name       string
		source     string
		wantParam  string
		wantSuffix string
	}{
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
			root, closeTree := mustParseTS(t, []byte(tt.source))
			defer closeTree()

			_, findings := computeTSFeatures(root, []byte(tt.source))

			got := mutatesInputFindingNamed(findings, tt.wantName, tt.evidence)
			if got == nil {
				t.Fatalf("computeTSFeatures for compound assignment %q: want mutates_input named %q with evidence %q, got %+v", tt.source, tt.wantName, tt.evidence, findings)
			}
		})
	}
}

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

// Story 2, known mutating method calls: each representative method
// (push, sort, set, delete-as-a-method) called on an identifier-bound
// parameter must yield a mutates_input Finding; an arbitrary custom method
// (setName) must not.
func TestTSMutatesInput_MutatingMethodCalls(t *testing.T) {
	tests := []struct {
		name      string
		source    string
		wantCount int
	}{
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
			root, closeTree := mustParseTS(t, []byte(tt.source))
			defer closeTree()

			_, findings := computeTSFeatures(root, []byte(tt.source))

			for _, f := range findings {
				if f.Kind == "mutates_input" {
					t.Fatalf("computeTSFeatures for %q: got mutates_input finding %+v, want none after parameter rebinding", tt.source, f)
				}
			}
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
			root, closeTree := mustParseTS(t, []byte(tt.source))
			defer closeTree()

			_, findings := computeTSFeatures(root, []byte(tt.source))

			for _, f := range findings {
				if f.Kind == "mutates_input" {
					t.Fatalf("computeTSFeatures for %q: got mutates_input finding %+v, want none because the write targets a block-local binding", tt.source, f)
				}
			}
		})
	}
}

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
			root, closeTree := mustParseTS(t, []byte(tt.source))
			defer closeTree()

			_, findings := computeTSFeatures(root, []byte(tt.source))

			for _, f := range findings {
				if f.Kind == "mutates_input" {
					t.Fatalf("computeTSFeatures for %q: got mutates_input finding %+v, want none because the write targets a binding that shadows the parameter", tt.source, f)
				}
			}
		})
	}
}

func TestTSMutatesInput_DestructuringAliasDoesNotShadowParameter(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
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
			root, closeTree := mustParseTS(t, []byte(tt.source))
			defer closeTree()

			_, findings := computeTSFeatures(root, []byte(tt.source))
			got := mustFindTSMutatesInput(t, tt.source, findings)

			if got.Name != "f:p" {
				t.Errorf("Finding.Name for %q = %q, want %q", tt.source, got.Name, "f:p")
			}
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

// AC6, shadowing: a nested function that redeclares a parameter with the
// same name as an outer function's parameter, and mutates its OWN
// parameter, must have the finding attributed to the INNER function, not
// the outer one.
func TestTSMutatesInput_ShadowedParameterAttributesToInnerOwner(t *testing.T) {
	source := `function outer(p) {
	function inner(p) {
		p.z = 3;
	}
	inner(p);
}
`
	root, closeTree := mustParseTS(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))
	got := mustFindTSMutatesInput(t, source, findings)

	if got.Name != "inner:p" {
		t.Errorf("Finding.Name = %q, want %q (inner function's own parameter shadows outer's same-named parameter)", got.Name, "inner:p")
	}
}

// Story 3, anonymous naming: an anonymous function expression whose
// identifier-bound parameter is mutated must use "anonymous@<start_byte>"
// for the Name's function half, not borrow a name from an enclosing
// variable_declarator.
func TestTSMutatesInput_AnonymousFunctionExpressionUsesStartByteName(t *testing.T) {
	source := `const f = function (p) {
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
			root, closeTree := mustParseTS(t, []byte(tt.source))
			defer closeTree()

			_, findings := computeTSFeatures(root, []byte(tt.source))
			for _, f := range findings {
				if f.Kind == "mutates_input" {
					t.Fatalf("computeTSFeatures for %q: got mutates_input finding %+v, want none (non-identifier-bound parameter must never be tracked)", tt.source, f)
				}
			}
		})
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
		})
	}
}

func TestTSMutatesInput_UpdateExpressionsMutateParameterRoots(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		wantName string
		evidence string
	}{
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
			root, closeTree := mustParseTS(t, []byte(tt.source))
			defer closeTree()

			_, findings := computeTSFeatures(root, []byte(tt.source))

			got := mutatesInputFindingNamed(findings, tt.wantName, tt.evidence)
			if got == nil {
				t.Fatalf("computeTSFeatures for update expression %q: want a mutates_input finding named %q with evidence %q, got %+v", tt.source, tt.wantName, tt.evidence, findings)
			}
		})
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

func TestTSMutatesInput_PredeclaredLocalBindingsShadowParameterForWholeScope(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
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
			root, closeTree := mustParseTS(t, []byte(tt.source))
			defer closeTree()

			_, findings := computeTSFeatures(root, []byte(tt.source))

			for _, f := range findings {
				if f.Kind == "mutates_input" {
					t.Fatalf("computeTSFeatures for predeclared local shadowing %q: got mutates_input finding %+v, want none because the write targets a local binding, not the parameter", tt.source, f)
				}
			}
		})
	}
}

func TestTSMutatesInput_VarParameterRebindDoesNotSuppressEarlierMutation(t *testing.T) {
	source := `function f(p) {
	p.x = 1;
	var p = {};
}
`
	root, closeTree := mustParseTS(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))
	got := mutatesInputFindingNamed(findings, "f:p", "p.x")
	if got == nil {
		t.Fatalf("computeTSFeatures for %q: want mutates_input before same-name var rebind, got %+v", source, findings)
	}
}

func TestTSMutatesInput_NoInitializerVarParameterDoesNotSuppressLaterMutation(t *testing.T) {
	source := `function f(p) {
	var p;
	p.x = 1;
}
`
	root, closeTree := mustParseTS(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))
	got := mutatesInputFindingNamed(findings, "f:p", "p.x")
	if got == nil {
		t.Fatalf("computeTSFeatures for %q: want mutates_input after no-initializer var parameter declaration, got %+v", source, findings)
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
	tests := []struct {
		name     string
		source   string
		wantName string
		evidence string
	}{
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
			root, closeTree := mustParseTS(t, []byte(tt.source))
			defer closeTree()

			_, findings := computeTSFeatures(root, []byte(tt.source))
			got := mutatesInputFindingNamed(findings, tt.wantName, tt.evidence)
			if got == nil {
				t.Fatalf("computeTSFeatures for destructuring assignment %q: want mutates_input named %q with evidence %q, got %+v", tt.source, tt.wantName, tt.evidence, findings)
			}
		})
	}
}

func TestTSMutatesInput_DestructuringAssignmentReadsDoNotCountAsTargets(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
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
			root, closeTree := mustParseTS(t, []byte(tt.source))
			defer closeTree()

			_, findings := computeTSFeatures(root, []byte(tt.source))
			for _, f := range findings {
				if f.Kind == "mutates_input" {
					t.Fatalf("computeTSFeatures for destructuring read %q: got mutates_input finding %+v, want none because p.x is read but not assigned", tt.source, f)
				}
			}
		})
	}
}

// mutatesInputFindingNamed returns the first mutates_input Finding in
// findings matching both name and evidence, or nil if none matches.
func mutatesInputFindingNamed(findings []Finding, name, evidence string) *Finding {
	for i := range findings {
		if findings[i].Kind == "mutates_input" && findings[i].Name == name && findings[i].Evidence == evidence {
			return &findings[i]
		}
	}
	return nil
}

// Regression guard: existing tight_coupling behavior must remain
// unaffected by mutates_input detection sharing the same walk -- a
// constructor's `this.x = new Y()` still yields exactly one tight_coupling
// finding and no mutates_input finding, even though the constructor also
// has an identifier-bound parameter.
func TestTSMutatesInput_DoesNotInterfereWithTightCoupling(t *testing.T) {
	source := `class C {
	constructor(cfg) {
		this.svc = new HttpClient(cfg);
	}
}
`
	root, closeTree := mustParseTS(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))

	var tightCoupling, mutatesInput int
	for _, f := range findings {
		switch f.Kind {
		case "tight_coupling":
			tightCoupling++
		case "mutates_input":
			mutatesInput++
		}
	}
	if tightCoupling != 1 {
		t.Errorf("tight_coupling findings = %d, want 1", tightCoupling)
	}
	if mutatesInput != 0 {
		t.Errorf("mutates_input findings = %d, want 0 (constructor body only reads cfg, never writes through it)", mutatesInput)
	}
}

// TSX variant of the property-assignment case, proving mutates_input works
// against a TSX-parsed tree the same way it does for TS (same pattern as
// TestComputeTSFeatures_WorksOnTSXParsedTree for the metrics half).
func TestTSXMutatesInput_PropertyAssignment(t *testing.T) {
	source := `const Widget = (props) => {
	props.value = 1;
	return null;
};
`
	root, closeTree := mustParseTSX(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))
	got := mustFindTSMutatesInput(t, source, findings)

	funcStart := len("const Widget = ")
	wantName := fmt.Sprintf("anonymous@%d:props", funcStart)
	if got.Name != wantName {
		t.Errorf("Finding.Name = %q, want %q", got.Name, wantName)
	}
	gotText := source[got.Location.StartByte:got.Location.EndByte]
	if gotText != "props.value" {
		t.Errorf("Finding.Location text = %q, want %q", gotText, "props.value")
	}
}

func TestTSXMutatesInput_CompoundAssignment(t *testing.T) {
	source := `const Widget = (props) => {
	props.value += 1;
	return null;
};
`
	root, closeTree := mustParseTSX(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))
	got := mutatesInputFindingNamed(findings, "anonymous@15:props", "props.value")
	if got == nil {
		t.Fatalf("computeTSFeatures for TSX compound assignment %q: want mutates_input named %q with evidence %q, got %+v", source, "anonymous@15:props", "props.value", findings)
	}
}
