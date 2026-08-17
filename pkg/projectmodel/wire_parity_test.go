package projectmodel

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/projectbridge"
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

// TestProtocolGoTSFieldParity guards the Go/TS sidecar wire-protocol
// invariant behind issue #216 Task 1: internal/projectbridge/protocol.go's
// Go Request/Response structs and js/semantics/src/project-sidecar/
// protocol.ts's TypeScript mirror must expose the same call-graph and
// possible-call-reachability-fact field names, in the same order, with
// matching optionality. protocol.ts is read as source text (Go cannot
// import a TypeScript package), so this is the only guard against the two
// drifting -- see assertGoFieldMatchesTS/tsInterfaceFields below for how a
// TS interface's fields are recovered without a TS parser.
//
// Fields are looked up dynamically by name (reflect.Type.FieldByName), not
// referenced as compile-time Go identifiers, so this test compiles and runs
// (and fails cleanly, not with a build error) before the wire types it
// checks for exist.
func TestProtocolGoTSFieldParity(t *testing.T) {
	tsSource := readProtocolTSSource(t)

	reqType := reflect.TypeOf(projectbridge.Request{})
	respType := reflect.TypeOf(projectbridge.Response{})

	assertGoTSStructFieldsMatch(t, reqType, tsSource, "Request")
	assertGoTSStructFieldsMatch(t, respType, tsSource, "Response")

	assertGoFieldMatchesTS(t, respType, "CallGraph", tsSource, "CallGraphEdgeFact")

	reachType := assertGoFieldMatchesTS(t, respType, "ReachabilityFacts", tsSource, "ReachabilityFactWire")
	assertGoFieldMatchesTS(t, reachType, "Path", tsSource, "ReachabilityStepFact")
}

// readProtocolTSSource reads js/semantics/src/project-sidecar/protocol.ts
// relative to this test file's own path, mirroring
// ts_sidecar_integration_acceptance_test.go's repoRootFromThisFile
// convention (that helper lives in the projectmodel_test package, so it is
// not reachable from here).
func readProtocolTSSource(t *testing.T) []byte {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	path := filepath.Join(repoRoot, "js", "semantics", "src", "project-sidecar", "protocol.ts")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %s", path, err)
	}
	return data
}

// assertGoTSStructFieldsMatch asserts structType's own json-tagged field
// names/optionality (in declaration order) match tsInterfaceName's fields in
// tsSource.
func assertGoTSStructFieldsMatch(t *testing.T, structType reflect.Type, tsSource []byte, tsInterfaceName string) {
	t.Helper()
	goFields := goWireFields(t, structType)
	tsFields := tsInterfaceFields(t, tsSource, tsInterfaceName)
	if !reflect.DeepEqual(goFields, tsFields) {
		t.Fatalf("%s json fields %v do not match protocol.ts interface %s fields %v", structType, goFields, tsInterfaceName, tsFields)
	}
}

// assertGoFieldMatchesTS finds fieldName on parent (resolving through at
// most one pointer/slice indirection to a struct type) and asserts its
// json-tagged fields match tsInterfaceName's fields in tsSource. It fails
// with a clear "no field" message -- not a compile error -- when fieldName
// does not exist on parent yet, which is the expected red-test state before
// this task's wire fields are added. It returns the resolved element struct
// type so callers can assert further nested fields (e.g. a Path field's own
// step type).
func assertGoFieldMatchesTS(t *testing.T, parent reflect.Type, fieldName string, tsSource []byte, tsInterfaceName string) reflect.Type {
	t.Helper()
	f, ok := parent.FieldByName(fieldName)
	if !ok {
		t.Fatalf("%s has no field %q; the wire-protocol call-graph/reachability/bypass fields (issue #216 Task 1) are not implemented yet", parent, fieldName)
	}
	elemType := f.Type
	for elemType.Kind() == reflect.Pointer || elemType.Kind() == reflect.Slice {
		elemType = elemType.Elem()
	}
	if elemType.Kind() != reflect.Struct {
		t.Fatalf("%s.%s resolves to non-struct type %s", parent, fieldName, elemType)
	}
	assertGoTSStructFieldsMatch(t, elemType, tsSource, tsInterfaceName)
	return elemType
}

// wireField is one struct field's json-wire identity: its json tag name
// (without ",omitempty") and whether that tag carries ",omitempty".
type wireField struct {
	Name     string
	Optional bool
}

// goWireFields returns structType's exported fields' wireField values in
// declaration order.
func goWireFields(t *testing.T, structType reflect.Type) []wireField {
	t.Helper()
	fields := make([]wireField, 0, structType.NumField())
	for i := 0; i < structType.NumField(); i++ {
		f := structType.Field(i)
		tag := f.Tag.Get("json")
		if tag == "" {
			t.Fatalf("%s field %q has no json tag", structType, f.Name)
		}
		name, opts, _ := strings.Cut(tag, ",")
		fields = append(fields, wireField{Name: name, Optional: strings.Contains(opts, "omitempty")})
	}
	return fields
}

// tsInterfaceFieldPattern matches one TS interface property declaration
// line: leading whitespace, an identifier, an optional "?", then ":".
var tsInterfaceFieldPattern = regexp.MustCompile(`(?m)^\s*([A-Za-z_][A-Za-z0-9_]*)(\??):`)

// tsInterfaceFields returns interfaceName's field list recovered from an
// "export interface Name { ... }" block in source. The body is captured
// with a negated class ([^}]*), so it stops at the first "}"; this assumes
// (true for protocol.ts today) no interface body contains a nested "}".
func tsInterfaceFields(t *testing.T, source []byte, interfaceName string) []wireField {
	t.Helper()
	blockPattern := regexp.MustCompile(`export interface ` + regexp.QuoteMeta(interfaceName) + `\s*\{([^}]*)\}`)
	m := blockPattern.FindSubmatch(source)
	if m == nil {
		t.Fatalf("protocol.ts: no %q interface found; the wire-protocol call-graph/reachability/bypass fields (issue #216 Task 1) are not implemented yet", interfaceName)
	}
	matches := tsInterfaceFieldPattern.FindAllSubmatch(m[1], -1)
	fields := make([]wireField, 0, len(matches))
	for _, fm := range matches {
		fields = append(fields, wireField{Name: string(fm[1]), Optional: string(fm[2]) == "?"})
	}
	return fields
}
