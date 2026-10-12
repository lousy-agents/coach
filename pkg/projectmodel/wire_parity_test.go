package projectmodel

import (
	"reflect"
	"regexp"
	"testing"

	"github.com/lousy-agents/coach/internal/projectbridge"
)

// wireField is one struct field's json-wire identity: its json tag name
// (without ",omitempty") and whether that tag carries ",omitempty".
type wireField struct {
	Name     string
	Optional bool
}

// tsInterfaceFieldPattern matches one TS interface property declaration
// line: leading whitespace, an identifier, an optional "?", then ":".
var tsInterfaceFieldPattern = regexp.MustCompile(`(?m)^\s*([A-Za-z_][A-Za-z0-9_]*)(\??):`)

// gapCodeConstPattern matches one `export const GAP_<NAME> = "<code>";` line
// in reachability-registry.ts.
var gapCodeConstPattern = regexp.MustCompile(`(?m)^export const GAP_[A-Z_]+ = "([a-z0-9_]+)";$`)

// TestProtocolGoTSFieldParity guards the Go/TS sidecar wire-protocol
// invariant behind issue #216 Task 1: internal/projectbridge/protocol.go's
// Go Request/Response structs and js/semantics/src/project-sidecar/
// protocol.ts's TypeScript mirror must expose the same call-graph and
// possible-call-reachability-fact field names, in the same order, with
// matching optionality. protocol.ts is read as source text (Go cannot
// import a TypeScript package), so this is the only guard against the two
// drifting -- see assertGoFieldMatchesTS/tsInterfaceFields for how a
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

	t.Run("Request", func(t *testing.T) {
		assertGoTSStructFieldsMatch(t, reqType, tsSource, "Request")
	})
	t.Run("Response", func(t *testing.T) {
		assertGoTSStructFieldsMatch(t, respType, tsSource, "Response")
	})
	t.Run("Response.CallGraph", func(t *testing.T) {
		assertGoFieldMatchesTS(t, respType, "CallGraph", tsSource, "CallGraphEdgeFact")
	})
	t.Run("Response.ReachabilityFacts", func(t *testing.T) {
		reachType := assertGoFieldMatchesTS(t, respType, "ReachabilityFacts", tsSource, "ReachabilityFactWire")
		assertGoFieldMatchesTS(t, reachType, "Path", tsSource, "ReachabilityStepFact")
	})
	t.Run("Response.RootScopes", func(t *testing.T) {
		assertGoFieldMatchesTS(t, respType, "RootScopes", tsSource, "RootScopeFact")
	})
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

// TestReachabilityGapDiagnosticCodeParity guards issue #216's coverage-honesty
// invariant: tsReachabilityGapDiagnosticCodes (ts_reachability.go) must list
// exactly the same diagnostic codes as js/semantics/src/project-sidecar/
// reachability-registry.ts's GAP_* constants. Go cannot import that
// TypeScript file, so it is read as source text (mirroring
// TestProtocolGoTSFieldParity's approach for protocol.ts) -- a code present
// on only one side means BuildTypeScriptReachability/BuildTypeScriptLayerBypass
// either silently report Coverage.Complete: true for a genuinely unverified
// hop (a code missing from the Go side) or over-report incompleteness for a
// hop that was actually fully resolved (a code missing from the TS side).
func TestReachabilityGapDiagnosticCodeParity(t *testing.T) {
	tsSource := readReachabilityRegistryTSSource(t)
	tsCodes := gapCodeConstPattern.FindAllSubmatch(tsSource, -1)
	if len(tsCodes) == 0 {
		t.Fatal("reachability-registry.ts: no GAP_* constants found; gapCodeConstPattern likely no longer matches the source")
	}

	tsSet := make(map[string]bool, len(tsCodes))
	for _, m := range tsCodes {
		tsSet[string(m[1])] = true
	}

	for code := range tsReachabilityGapDiagnosticCodes {
		if !tsSet[code] {
			t.Errorf("tsReachabilityGapDiagnosticCodes (ts_reachability.go) has %q, but reachability-registry.ts has no matching GAP_* constant", code)
		}
	}
	for code := range tsSet {
		if !tsReachabilityGapDiagnosticCodes[code] {
			t.Errorf("reachability-registry.ts declares GAP_* constant %q, but tsReachabilityGapDiagnosticCodes (ts_reachability.go) does not include it", code)
		}
	}
}
