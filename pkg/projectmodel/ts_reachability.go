package projectmodel

import (
	"context"
	"io/fs"
)

// TSReachabilityAlgorithm identifies the TypeScript sidecar's own
// possible-call-reachability traversal, matching the exact string
// js/semantics/src/project-sidecar/reachability-registry.ts's
// REACHABILITY_ALGORITHM constant emits on every ReachabilityFactWire --
// Go and TypeScript must report the same algorithm-version string only
// when describing the same wire-produced facts, not two independently
// Go-invented identifiers. It is distinct from Go's own ReachabilityAlgorithm
// (go-source-sink-registry@1): the two traversal implementations evolve
// independently.
const TSReachabilityAlgorithm = "ts-source-sink-registry@1"

// BuildTypeScriptReachability builds a ReachabilityResult from the same TS
// sidecar round trip BuildTypeScriptModelViaSidecar makes -- it calls that
// function once and re-shapes its Model.ReachabilityFacts/Model.Coverage
// rather than re-implementing sidecar transport or issuing a second request.
// A caller that wants both a full Model (workspace/import/call facts) and a
// ReachabilityResult must call both functions separately and so pays for two
// independent sidecar round trips, mirroring how BuildGoModel and
// BuildGoReachability are also two independent builds on the Go side.
//
// Sources is derived as the deduplicated, sorted set of Facts[i].Source
// values -- a narrower claim than BuildGoReachability's Sources, which lists
// every registry source function identified in the snapshot even when it
// reached no sink. The TS wire protocol has no equivalent to that
// independent walk: the sidecar only emits a ReachabilityFactWire for a call
// that actually resolved to a sink, so a TS source with no resolved sink
// never appears here, unlike on the Go side.
func BuildTypeScriptReachability(ctx context.Context, snapshot fs.FS, meta SnapshotMeta, opts TSSidecarOptions) (ReachabilityResult, error) {
	model, err := BuildTypeScriptModelViaSidecar(ctx, snapshot, meta, opts)
	if err != nil {
		return ReachabilityResult{}, err
	}

	return ReachabilityResult{
		Facts:     model.ReachabilityFacts,
		Sources:   tsReachabilitySources(model.ReachabilityFacts),
		Algorithm: TSReachabilityAlgorithm,
		Coverage:  tsReachabilityCoverage(model.Coverage),
	}, nil
}

func tsReachabilitySources(facts []ReachabilityFact) []string {
	seen := map[string]bool{}
	for _, f := range facts {
		seen[f.Source] = true
	}
	return mapKeysSorted(seen)
}

// tsReachabilityGapDiagnosticCodes lists every diagnostic code
// js/semantics/src/project-sidecar/reachability.ts's recordGapDiagnostic
// call sites can emit (see that file's gapDiagnosticInfo and
// handleCallInSource) -- kept in lockstep with those literal strings, the
// same way TSReachabilityAlgorithm mirrors REACHABILITY_ALGORITHM. Each one
// means "this hop was deliberately left unverified by the depth-1 walk,"
// never an import/config/budget failure, so Model.Coverage.Complete (what
// internal/codesignalcli/project_ts_backend.go publishes as CLI
// ProjectCoverage) does not flip on their presence -- see analyze.ts's
// runProjects. Reachability's own completeness is derived from them here
// instead.
var tsReachabilityGapDiagnosticCodes = map[string]bool{
	"ts_reachability_dynamic_import_gap":          true,
	"ts_reachability_unresolved_handler_gap":      true,
	"ts_reachability_local_call_not_followed_gap": true,
	"ts_reachability_type_only_gap":               true,
	"ts_reachability_unresolved_type_gap":         true,
}

// tsReachabilityHasGap reports whether diagnostics contains any reachability
// coverage-gap diagnostic (see tsReachabilityGapDiagnosticCodes).
func tsReachabilityHasGap(diagnostics []Diagnostic) bool {
	for _, d := range diagnostics {
		if tsReachabilityGapDiagnosticCodes[d.Code] {
			return true
		}
	}
	return false
}

// tsReachabilityCoverage derives a reachability-specific Coverage from
// modelCoverage: Complete is additionally false whenever a reachability
// coverage-gap diagnostic is present, even though that diagnostic no longer
// flips modelCoverage.Complete itself (see tsReachabilityGapDiagnosticCodes).
// Diagnostics/Counts/Budgets/Phase are passed through unchanged.
func tsReachabilityCoverage(modelCoverage Coverage) Coverage {
	coverage := modelCoverage
	coverage.Complete = coverage.Complete && !tsReachabilityHasGap(coverage.Diagnostics)
	return coverage
}
