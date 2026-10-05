package projectmodel

import (
	"reflect"

	"testing"

	"github.com/lousy-agents/coach/internal/projectbridge"
)

func body_wireParityTest_ModelAndModelWireFieldsMirror11_42(t *testing.T, modelType reflect.Type, wireType reflect.Type) {
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

func body_wireParityTest_RootScopesElementTypeMirrorsRootScopeFact_60(t *testing.T, modelType reflect.Type) {
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
