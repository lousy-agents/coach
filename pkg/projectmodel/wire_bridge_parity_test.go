package projectmodel

import (
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/projectbridge"
)

// TestCoverageBridgeFieldParity guards the invariant documented on
// projectbridge.Coverage: it mirrors Coverage's exact field name/order/json
// tags because the two types cannot share an identity (importing
// projectmodel from internal/projectbridge would create a cycle). Nothing
// else fails at compile time if the two types drift, so this reflection
// check is the only guard.
func TestCoverageBridgeFieldParity(t *testing.T) {
	assertFieldParity(t, reflect.TypeOf(Coverage{}), reflect.TypeOf(projectbridge.Coverage{}))
}

// TestDiagnosticBridgeFieldParity is TestCoverageBridgeFieldParity's
// counterpart for Diagnostic/projectbridge.Diagnostic.
func TestDiagnosticBridgeFieldParity(t *testing.T) {
	assertFieldParity(t, reflect.TypeOf(Diagnostic{}), reflect.TypeOf(projectbridge.Diagnostic{}))
}

func assertFieldParity(t *testing.T, want, got reflect.Type) {
	t.Helper()

	if want.NumField() != got.NumField() {
		t.Fatalf("%s has %d fields but %s has %d fields; every field must be mirrored in name, order, and json tag", want, want.NumField(), got, got.NumField())
	}

	for i := 0; i < want.NumField(); i++ {
		wantField := want.Field(i)
		gotField := got.Field(i)

		if wantField.Name != gotField.Name {
			t.Fatalf("field %d: %s has %q but %s has %q; fields must mirror in name and order", i, want, wantField.Name, got, gotField.Name)
		}
		if wantJSON, gotJSON := wantField.Tag.Get("json"), gotField.Tag.Get("json"); wantJSON != gotJSON {
			t.Fatalf("field %q: %s json tag %q does not match %s json tag %q", wantField.Name, got, gotJSON, want, wantJSON)
		}
	}
}
