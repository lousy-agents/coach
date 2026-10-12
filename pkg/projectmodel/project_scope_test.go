package projectmodel

import (
	"regexp"
	"testing"
)

// reachabilityAlgorithmConstPattern matches reachability-registry.ts's
// `export const REACHABILITY_ALGORITHM = "<value>";` declaration.
var reachabilityAlgorithmConstPattern = regexp.MustCompile(`(?m)^export const REACHABILITY_ALGORITHM = "([^"]+)";$`)

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
