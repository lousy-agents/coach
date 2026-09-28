package projectmodel

import (
	"reflect"
	"regexp"

	"testing"

	"github.com/lousy-agents/coach/internal/projectbridge"
	"github.com/lousy-agents/coach/pkg/domain"
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

// reachabilityAlgorithmConstPattern matches reachability-registry.ts's
// `export const REACHABILITY_ALGORITHM = "<value>";` declaration.
var reachabilityAlgorithmConstPattern = regexp.MustCompile(`(?m)^export const REACHABILITY_ALGORITHM = "([^"]+)";$`)

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
		body_wireParityTest_ModelAndModelWireFieldsMirror11_42(t, modelType, wireType)
	})

	t.Run("RootScopes element type mirrors RootScopeFact", func(t *testing.T) {
		body_wireParityTest_RootScopesElementTypeMirrorsRootScopeFact_60(t, modelType)
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
