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
