package codesignalcli

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestPolicyDeclarationsReadFromTheDocument(t *testing.T) {
	cases := []struct {
		name          string
		config        json.RawMessage
		wantRoots     []string
		wantForbidden bool
	}{
		{"no document", nil, nil, false},
		{"a document that does not parse", json.RawMessage(`{`), nil, false},
		{"roots and no forbidden imports", json.RawMessage(`{"schema_version":"1","roots":["a","b"]}`), []string{"a", "b"}, false},
		{"roots and a forbidden import", json.RawMessage(`{"schema_version":"1","roots":["."],"layers":[{"name":"x","prefixes":["x"]},{"name":"y","prefixes":["y"]}],"forbidden_imports":[{"from":"x","to":"y"}]}`), []string{"."}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertPolicyDeclarations(t, c.config, c.wantRoots, c.wantForbidden)
		})
	}
}

func assertPolicyDeclarations(t *testing.T, config json.RawMessage, wantRoots []string, wantForbidden bool) {
	t.Helper()
	if got := selectedRootsFromConfig(config); !reflect.DeepEqual(got, wantRoots) {
		t.Errorf("selectedRootsFromConfig = %#v, want %#v", got, wantRoots)
	}
	if got := forbiddenImportsDeclared(config); got != wantForbidden {
		t.Errorf("forbiddenImportsDeclared = %v, want %v", got, wantForbidden)
	}
}
