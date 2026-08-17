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

	assertFieldParityPrefix(t, want, got)
}

// assertFieldParityPrefix asserts want's fields are mirrored, in name,
// order, and json tag, by got's first want.NumField() fields. Unlike
// assertFieldParity, it does not require got to have exactly want's field
// count, so it also covers a projectbridge wire type that adds a trailing
// Backend provenance field with no projectmodel counterpart.
func assertFieldParityPrefix(t *testing.T, want, got reflect.Type) {
	t.Helper()

	if got.NumField() < want.NumField() {
		t.Fatalf("%s has %d fields but %s has only %d fields; every %s field must be mirrored, in order, as a prefix of %s's fields", want, want.NumField(), got, got.NumField(), want, got)
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

// TestReachabilityBypassWireFieldParity is TestCoverageBridgeFieldParity's
// counterpart for the call-graph/reachability wire types: each projectbridge
// type here adds a trailing Backend provenance field with no projectmodel
// counterpart, so this uses assertFieldParityPrefix rather than
// assertFieldParity.
func TestReachabilityBypassWireFieldParity(t *testing.T) {
	assertFieldParityPrefix(t, reflect.TypeOf(CallFact{}), reflect.TypeOf(projectbridge.CallGraphEdgeFact{}))
	assertFieldParityPrefix(t, reflect.TypeOf(ReachabilityStep{}), reflect.TypeOf(projectbridge.ReachabilityStepFact{}))
	assertFieldParityPrefix(t, reflect.TypeOf(ReachabilityFact{}), reflect.TypeOf(projectbridge.ReachabilityFactWire{}))
}
