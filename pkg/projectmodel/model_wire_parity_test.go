package projectmodel

import (
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/projectbridge"
	"github.com/lousy-agents/coach/pkg/domain"
)

// TestModelWireFieldParity guards the invariant documented on modelWire:
// every field of Model must be mirrored in modelWire in the same order and
// under the same json tag, or it is silently dropped from JSON output. This
// is an internal (package-private) unit test rather than an acceptance
// test because modelWire is unexported and the invariant it protects is an
// implementation detail, not externally observable behavior.
func TestModelWireFieldParity(t *testing.T) {
	modelType := reflect.TypeOf(Model{})
	wireType := reflect.TypeOf(domain.ModelWire{})

	t.Run("Model and modelWire fields mirror 1:1", func(t *testing.T) {
		assertModelFieldsMirroredByWire(t, modelType, wireType)
	})

	t.Run("RootScopes element type mirrors RootScopeFact", func(t *testing.T) {
		assertRootScopeMirroredByBridgeFact(t, modelType)
	})
}

func assertModelFieldsMirroredByWire(t *testing.T, modelType reflect.Type, wireType reflect.Type) {
	if modelType.NumField() != wireType.NumField() {
		t.Fatalf("Model has %d fields but modelWire has %d fields; every Model field must be mirrored in modelWire (see modelWire's doc comment)", modelType.NumField(), wireType.NumField())
	}

	for i := 0; i < modelType.NumField(); i++ {
		modelField := modelType.Field(i)
		wireField := wireType.Field(i)

		if modelField.Name != wireField.Name {
			t.Errorf("field %d: Model has %q but modelWire has %q; fields must mirror in name and order", i, modelField.Name, wireField.Name)
		}
		if got, want := wireField.Tag.Get("json"), modelField.Tag.Get("json"); got != want {
			t.Errorf("field %q: modelWire json tag %q does not match Model json tag %q", modelField.Name, got, want)
		}
	}
}

func assertRootScopeMirroredByBridgeFact(t *testing.T, modelType reflect.Type) {
	rootScopesField, ok := modelType.FieldByName("RootScopes")
	if !ok {
		t.Fatal("Model has no RootScopes field")
	}
	if rootScopesField.Type.Kind() != reflect.Slice {
		t.Fatalf("Model.RootScopes has type %s, want a slice", rootScopesField.Type)
	}
	rootScopeType := rootScopesField.Type.Elem()
	bridgeRootScopeType := reflect.TypeOf(projectbridge.RootScopeFact{})
	if rootScopeType.NumField() != bridgeRootScopeType.NumField() {
		t.Fatalf("Model's root scope element type %s has %d fields but projectbridge.RootScopeFact has %d fields; they must mirror 1:1", rootScopeType, rootScopeType.NumField(), bridgeRootScopeType.NumField())
	}
	for i := 0; i < rootScopeType.NumField(); i++ {
		got := rootScopeType.Field(i)
		want := bridgeRootScopeType.Field(i)
		if got.Name != want.Name {
			t.Errorf("field %d: Model's root scope element type has %q but projectbridge.RootScopeFact has %q", i, got.Name, want.Name)
		}
		if gotTag, wantTag := got.Tag.Get("json"), want.Tag.Get("json"); gotTag != wantTag {
			t.Errorf("field %q: Model's root scope element type json tag %q does not match projectbridge.RootScopeFact json tag %q", got.Name, gotTag, wantTag)
		}
	}
}
