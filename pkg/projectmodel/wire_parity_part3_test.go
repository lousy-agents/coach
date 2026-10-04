package projectmodel

import (
	"os"
	"path/filepath"
	"reflect"

	"runtime"

	"testing"
)

// TestReachabilityAlgorithmWireParity guards SA-280-014 (issue #332 Task 9,
// AC-8/AC-19/AC-27): ProjectScope.PatternSet (project_scope.go) is assigned
// from TSReachabilityAlgorithm, which must keep mirroring
// js/semantics/src/project-sidecar/reachability-registry.ts's
// REACHABILITY_ALGORITHM constant -- the route-to-sink registry's own
// identity -- the same cross-language text-read approach
// TestReachabilityGapDiagnosticCodeParity uses for GAP_* codes. If the TS
// registry's algorithm identity changed without a matching bump on the Go
// side, pattern_set would silently keep reporting a stale registry version.
//
// pattern_set must never be sourced from SchemaVersion (the Model's own
// wire-schema version, and the shape a future project_provenance.analyzer.
// version field would carry): the two are independently-versioned identity
// dimensions -- one names the route-to-sink registry, the other names the
// project model's wire schema -- and ProjectScopeFromModel must never
// accidentally alias PatternSet to it.
func TestReachabilityAlgorithmWireParity(t *testing.T) {
	tsSource := readReachabilityRegistryTSSource(t)
	m := reachabilityAlgorithmConstPattern.FindSubmatch(tsSource)
	if m == nil {
		t.Fatal("reachability-registry.ts: no REACHABILITY_ALGORITHM constant found; reachabilityAlgorithmConstPattern likely no longer matches the source")
	}
	tsAlgorithm := string(m[1])

	if tsAlgorithm != TSReachabilityAlgorithm {
		t.Errorf("reachability-registry.ts REACHABILITY_ALGORITHM = %q, but projectmodel.TSReachabilityAlgorithm = %q; ProjectScope.PatternSet (project_scope.go) is assigned from TSReachabilityAlgorithm, so the two must match", tsAlgorithm, TSReachabilityAlgorithm)
	}

	if TSReachabilityAlgorithm == SchemaVersion {
		t.Fatalf("TSReachabilityAlgorithm (%q) must not equal SchemaVersion (%q): pattern_set (the route-to-sink registry identity) and the project model's wire-schema version are independently-sourced identity dimensions, never one constant standing in for both", TSReachabilityAlgorithm, SchemaVersion)
	}

	policy := ProjectScopePolicy{Roots: []string{"."}}
	model := Model{
		SchemaVersion: SchemaVersion,
		RootScopes:    []RootScope{{Root: ".", CandidateFiles: 1, AnalyzedFiles: 1}},
	}
	scope, err := ProjectScopeFromModel(model, policy)
	if err != nil {
		t.Fatalf("ProjectScopeFromModel: %s", err)
	}
	if scope.PatternSet != TSReachabilityAlgorithm {
		t.Errorf("ProjectScope.PatternSet = %q, want TSReachabilityAlgorithm (%q)", scope.PatternSet, TSReachabilityAlgorithm)
	}
	if scope.PatternSet == model.SchemaVersion {
		t.Errorf("ProjectScope.PatternSet (%q) must not equal Model.SchemaVersion (%q): pattern_set is never derived from the wire-schema version", scope.PatternSet, model.SchemaVersion)
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
