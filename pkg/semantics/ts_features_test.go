package semantics

import "testing"

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
