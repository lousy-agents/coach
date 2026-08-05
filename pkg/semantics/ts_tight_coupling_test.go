package semantics

import (
	"testing"
)

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
