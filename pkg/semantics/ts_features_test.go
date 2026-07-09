package semantics

import (
	"fmt"
	"testing"
)

// AC-R4.1: computeTSFeatures must count exactly one StructuralMetrics field
// per tracked node kind, with TypeSwitches/Selects always 0 (no TS analog).
// The fixture hand-counts to 2 ifs, 1 for, 1 for...of, 1 switch, 1 top-level
// function, and 1 class method.
func TestComputeTSFeatures_CountsEachNodeKindExactly(t *testing.T) {
	source := []byte(`function f(x: number) {
	if (x > 0) {
	}
	if (x < 0) {
	}
	for (let i = 0; i < x; i++) {
	}
	for (const v of [1, 2, 3]) {
	}
	switch (x) {
		case 1:
			break;
	}
}

class C {
	method() {
	}
}
`)
	root, closeTree := mustParseTS(t, source)
	defer closeTree()

	metrics, _ := computeTSFeatures(root, source)

	want := StructuralMetrics{
		Ifs:          2,
		Fors:         2,
		ExprSwitches: 1,
		Functions:    1,
		Methods:      1,
	}
	if metrics.Ifs != want.Ifs {
		t.Errorf("computeTSFeatures Ifs: got %d, want %d", metrics.Ifs, want.Ifs)
	}
	if metrics.Fors != want.Fors {
		t.Errorf("computeTSFeatures Fors: got %d, want %d", metrics.Fors, want.Fors)
	}
	if metrics.ExprSwitches != want.ExprSwitches {
		t.Errorf("computeTSFeatures ExprSwitches: got %d, want %d", metrics.ExprSwitches, want.ExprSwitches)
	}
	if metrics.Functions != want.Functions {
		t.Errorf("computeTSFeatures Functions: got %d, want %d", metrics.Functions, want.Functions)
	}
	if metrics.Methods != want.Methods {
		t.Errorf("computeTSFeatures Methods: got %d, want %d", metrics.Methods, want.Methods)
	}
	if metrics.TypeSwitches != 0 {
		t.Errorf("computeTSFeatures TypeSwitches: got %d, want 0 (no TypeScript analog)", metrics.TypeSwitches)
	}
	if metrics.Selects != 0 {
		t.Errorf("computeTSFeatures Selects: got %d, want 0 (no TypeScript analog)", metrics.Selects)
	}
}

// AC-R4.2: MaxNestingDepth counts one level per braced statement_block
// within a function body (the body's own braces are depth 1), and a
// brace-less body contributes no additional depth.
func TestComputeTSFeatures_MaxNestingDepth(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   int
	}{
		{
			name:   "two braced nested ifs",
			source: "function f() { if (a) { if (b) { } } }",
			want:   3,
		},
		{
			name:   "brace-less inner if",
			source: "function f() { if (a) { if (b) g(); } }",
			want:   2,
		},
		{
			name:   "arrow function with expression body contributes zero depth",
			source: "const f = (x: number) => x + 1;",
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, closeTree := mustParseTS(t, []byte(tt.source))
			defer closeTree()

			metrics, _ := computeTSFeatures(root, []byte(tt.source))

			if metrics.MaxNestingDepth != tt.want {
				t.Errorf("computeTSFeatures MaxNestingDepth for %q: got %d, want %d", tt.source, metrics.MaxNestingDepth, tt.want)
			}
		})
	}
}

// AC-R4.5: a file whose only function-like node is an arrow_function must
// report Functions == 1 and Methods == 0, confirming arrows count toward
// Functions.
func TestComputeTSFeatures_ArrowFunctionCountsAsFunctionNotMethod(t *testing.T) {
	source := []byte(`const f = (x: number) => { return x; };`)
	root, closeTree := mustParseTS(t, source)
	defer closeTree()

	metrics, _ := computeTSFeatures(root, source)

	if metrics.Functions != 1 {
		t.Errorf("computeTSFeatures for arrow-only file %q: Functions = %d, want 1", source, metrics.Functions)
	}
	if metrics.Methods != 0 {
		t.Errorf("computeTSFeatures for arrow-only file %q: Methods = %d, want 0", source, metrics.Methods)
	}
}

// AC-R4.3: `this.x = new Y(...)` inside a constructor emits exactly one
// tight_coupling Finding, named after the constructor callee, spanning the
// new_expression.
func TestComputeTSFeatures_TightCouplingInConstructor(t *testing.T) {
	source := []byte(`class C {
	constructor() {
		this.svc = new HttpClient("http://x");
	}
}
`)
	root, closeTree := mustParseTS(t, source)
	defer closeTree()

	_, findings := computeTSFeatures(root, source)

	if len(findings) != 1 {
		t.Fatalf("computeTSFeatures for %q: got %d findings (%+v), want exactly 1", source, len(findings), findings)
	}
	got := findings[0]
	if got.Kind != "tight_coupling" {
		t.Errorf("computeTSFeatures for %q: Finding.Kind = %q, want %q", source, got.Kind, "tight_coupling")
	}
	if got.Name != "HttpClient" {
		t.Errorf("computeTSFeatures for %q: Finding.Name = %q, want %q", source, got.Name, "HttpClient")
	}
	wantText := `new HttpClient("http://x")`
	gotText := string(source[got.Location.StartByte:got.Location.EndByte])
	if gotText != wantText {
		t.Errorf("computeTSFeatures for %q: Finding.Location text = %q, want %q", source, gotText, wantText)
	}
}

// AC-R4.3 (descendants): a tight-coupling assignment nested inside an `if`
// within the constructor body must still be found.
func TestComputeTSFeatures_TightCouplingNestedInsideConstructorIf(t *testing.T) {
	source := []byte(`class C {
	constructor(flag: boolean) {
		if (flag) {
			this.other = new Other();
		}
	}
}
`)
	root, closeTree := mustParseTS(t, source)
	defer closeTree()

	_, findings := computeTSFeatures(root, source)

	if len(findings) != 1 {
		t.Fatalf("computeTSFeatures for %q: got %d findings (%+v), want exactly 1", source, len(findings), findings)
	}
	if findings[0].Name != "Other" {
		t.Errorf("computeTSFeatures for %q: Finding.Name = %q, want %q", source, findings[0].Name, "Other")
	}
}

// AC-R4.4: a new_expression outside any constructor, and a variable
// initializer (`const c = new X()`) inside a constructor, must each yield
// no tight_coupling finding.
func TestComputeTSFeatures_ExcludesNonMatchingNewExpressions(t *testing.T) {
	source := []byte(`class C {
	constructor() {
		const c = new NotCounted();
	}

	method() {
		this.x = new AlsoNotCounted();
	}
}
`)
	root, closeTree := mustParseTS(t, source)
	defer closeTree()

	_, findings := computeTSFeatures(root, source)

	if len(findings) != 0 {
		t.Errorf("computeTSFeatures for %q: got %d findings (%+v), want 0 (variable_declarator initializers and non-constructor methods are excluded)", source, len(findings), findings)
	}
}

// Findings must be emitted in ascending Location.StartByte order across
// multiple constructors in one file.
func TestComputeTSFeatures_OrdersFindingsByStartByteAscendingAcrossConstructors(t *testing.T) {
	source := []byte(`class First {
	constructor() {
		this.a = new A();
	}
}

class Second {
	constructor() {
		this.b = new B();
	}
}
`)
	root, closeTree := mustParseTS(t, source)
	defer closeTree()

	_, findings := computeTSFeatures(root, source)

	if len(findings) != 2 {
		t.Fatalf("computeTSFeatures for %q: got %d findings (%+v), want exactly 2", source, len(findings), findings)
	}
	if findings[0].Name != "A" || findings[1].Name != "B" {
		t.Fatalf("computeTSFeatures for %q: findings = %+v, want A before B (source order)", source, findings)
	}
	if findings[0].Location.StartByte >= findings[1].Location.StartByte {
		t.Errorf("computeTSFeatures for %q: findings must be ordered by Location.StartByte ascending, got %+v", source, findings)
	}
}

// Regression guard raised by review (Copilot): tight_coupling must only
// match assignments to a property of `this` (this.<prop> or
// this[<expr>]), not a plain variable or another object's property, even
// though both also have a new_expression on the right inside a
// constructor.
func TestComputeTSFeatures_ExcludesNonThisAssignments(t *testing.T) {
	source := []byte(`class C {
	constructor() {
		x = new X();
		other.y = new Y();
	}
}
`)
	root, closeTree := mustParseTS(t, source)
	defer closeTree()

	_, findings := computeTSFeatures(root, source)

	if len(findings) != 0 {
		t.Errorf("computeTSFeatures for %q: got %d findings (%+v), want 0 (assignments not targeting `this` are not tight coupling)", source, len(findings), findings)
	}
}

// Regression guard raised by review (Copilot): `this[<expr>] = new Y()`
// (a subscript_expression on `this`) must still be matched, not just the
// member_expression form `this.<prop> = new Y()`.
func TestComputeTSFeatures_IncludesThisSubscriptAssignment(t *testing.T) {
	source := []byte(`class C {
	constructor() {
		this['svc'] = new HttpClient();
	}
}
`)
	root, closeTree := mustParseTS(t, source)
	defer closeTree()

	_, findings := computeTSFeatures(root, source)

	if len(findings) != 1 {
		t.Fatalf("computeTSFeatures for %q: got %d findings (%+v), want exactly 1", source, len(findings), findings)
	}
	if findings[0].Name != "HttpClient" {
		t.Errorf("computeTSFeatures for %q: Finding.Name = %q, want %q", source, findings[0].Name, "HttpClient")
	}
}

// Regression guard raised by review: a class declared inside a constructor
// body, with its own constructor doing `this.x = new Y()`, must be
// reported exactly once (by its own method_definition visit), not also by
// the enclosing constructor's scan treating the nested class as a plain
// descendant.
func TestComputeTSFeatures_DoesNotDuplicateFindingForNestedClassConstructor(t *testing.T) {
	source := []byte(`class Outer {
	constructor() {
		class Inner {
			constructor() {
				this.svc = new HttpClient();
			}
		}
		new Inner();
	}
}
`)
	root, closeTree := mustParseTS(t, source)
	defer closeTree()

	_, findings := computeTSFeatures(root, source)

	if len(findings) != 1 {
		t.Fatalf("computeTSFeatures for %q: got %d findings (%+v), want exactly 1 (the nested constructor's assignment must not be reported twice)", source, len(findings), findings)
	}
	if findings[0].Name != "HttpClient" {
		t.Errorf("computeTSFeatures for %q: Finding.Name = %q, want %q", source, findings[0].Name, "HttpClient")
	}
}

// Regression guard raised by review: a plain (non-arrow) function nested
// inside a constructor has its own `this` binding, unrelated to the class
// instance, so an assignment inside it must not be misattributed to the
// enclosing constructor as tight_coupling.
func TestComputeTSFeatures_ExcludesAssignmentInsideNestedPlainFunction(t *testing.T) {
	source := []byte(`class C {
	constructor() {
		function helper() {
			this.svc = new HttpClient();
		}
		helper();
	}
}
`)
	root, closeTree := mustParseTS(t, source)
	defer closeTree()

	_, findings := computeTSFeatures(root, source)

	if len(findings) != 0 {
		t.Errorf("computeTSFeatures for %q: got %d findings (%+v), want 0 (a nested plain function's `this` is not the constructor's instance)", source, len(findings), findings)
	}
}

// Regression guard: an arrow function nested inside a constructor does NOT
// rebind `this`, so an assignment inside it must still be attributed to
// the enclosing constructor as tight_coupling.
func TestComputeTSFeatures_IncludesAssignmentInsideNestedArrowFunction(t *testing.T) {
	source := []byte(`class C {
	constructor() {
		const setup = () => {
			this.svc = new HttpClient();
		};
		setup();
	}
}
`)
	root, closeTree := mustParseTS(t, source)
	defer closeTree()

	_, findings := computeTSFeatures(root, source)

	if len(findings) != 1 {
		t.Fatalf("computeTSFeatures for %q: got %d findings (%+v), want exactly 1 (arrow functions inherit `this` from the enclosing constructor)", source, len(findings), findings)
	}
	if findings[0].Name != "HttpClient" {
		t.Errorf("computeTSFeatures for %q: Finding.Name = %q, want %q", source, findings[0].Name, "HttpClient")
	}
}

// AC-R2.3 (metrics half): computeTSFeatures must behave identically when
// given a TSX-parsed tree, proving the shared extractor works across both
// grammars (Node.Kind() strings, which the walk matches on, resolve
// against each node's own tree language regardless of which grammar parsed
// it).
func TestComputeTSFeatures_WorksOnTSXParsedTree(t *testing.T) {
	source := []byte(`const App = () => {
	if (true) {
		return null;
	}
	return null;
};
`)
	root, closeTree := mustParseTSX(t, source)
	defer closeTree()

	metrics, _ := computeTSFeatures(root, source)

	if metrics.Functions != 1 {
		t.Errorf("computeTSFeatures on TSX tree for %q: Functions = %d, want 1", source, metrics.Functions)
	}
	if metrics.Ifs != 1 {
		t.Errorf("computeTSFeatures on TSX tree for %q: Ifs = %d, want 1", source, metrics.Ifs)
	}
	if metrics.MaxNestingDepth != 2 {
		t.Errorf("computeTSFeatures on TSX tree for %q: MaxNestingDepth = %d, want 2", source, metrics.MaxNestingDepth)
	}
}

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
		{name: "push", source: "function f(arr) {\n\tarr.push(1);\n}\n", wantCount: 1},
		{name: "sort", source: "function f(arr) {\n\tarr.sort();\n}\n", wantCount: 1},
		{name: "set", source: "function f(m) {\n\tm.set('k', 1);\n}\n", wantCount: 1},
		{name: "delete method", source: "function f(m) {\n\tm.delete('k');\n}\n", wantCount: 1},
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
