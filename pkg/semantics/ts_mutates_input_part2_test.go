package semantics

import (
	"testing"
)

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
			body_tsMutatesInputPart2Test_82(t, tt)
		})
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
