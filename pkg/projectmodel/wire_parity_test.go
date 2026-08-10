package projectmodel

import (
	"reflect"
	"testing"
)

// TestModelWireFieldParity guards the invariant documented on modelWire:
// every field of Model must be mirrored in modelWire in the same order and
// under the same json tag, or it is silently dropped from JSON output. This
// is an internal (package-private) unit test rather than an acceptance
// test because modelWire is unexported and the invariant it protects is an
// implementation detail, not externally observable behavior.
func TestModelWireFieldParity(t *testing.T) {
	modelType := reflect.TypeOf(Model{})
	wireType := reflect.TypeOf(modelWire{})

	if modelType.NumField() != wireType.NumField() {
		t.Fatalf("Model has %d fields but modelWire has %d fields; every Model field must be mirrored in modelWire (see modelWire's doc comment)", modelType.NumField(), wireType.NumField())
	}

	for i := 0; i < modelType.NumField(); i++ {
		modelField := modelType.Field(i)
		wireField := wireType.Field(i)

		if modelField.Name != wireField.Name {
			t.Fatalf("field %d: Model has %q but modelWire has %q; fields must mirror in name and order", i, modelField.Name, wireField.Name)
		}
		if got, want := wireField.Tag.Get("json"), modelField.Tag.Get("json"); got != want {
			t.Fatalf("field %q: modelWire json tag %q does not match Model json tag %q", modelField.Name, got, want)
		}
	}
}
