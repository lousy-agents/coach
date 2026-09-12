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

// Deprecated: call BuildTypeScriptModelViaSidecar once and pass the Model to
// BuildTypeScriptReachabilityFromModel / BuildTypeScriptLayerBypassFromModel
// instead, so multiple derivations share one sidecar round trip.
func BuildTypeScriptReachability(ctx context.Context, snapshot fs.FS, meta SnapshotMeta, opts TSSidecarOptions) (ReachabilityResult, error) {
	model, err := BuildTypeScriptModelViaSidecar(ctx, snapshot, meta, opts)
	if err != nil {
		return ReachabilityResult{}, err
	}
	return BuildTypeScriptReachabilityFromModel(model), nil
}

func BuildTypeScriptReachabilityFromModel(model Model) ReachabilityResult {
	return ReachabilityResult{
		Facts:     model.ReachabilityFacts,
		Sources:   tsReachabilitySourcesReachedASink(model.ReachabilityFacts),
		Algorithm: TSReachabilityAlgorithm,
		Coverage:  tsReachabilityCoverage(model.Coverage),
	}
}

// tsReachabilitySourcesReachedASink is narrower than BuildGoReachability's
// Sources, which lists every registry source function found in the
// snapshot even when it reached no sink: the sidecar only emits a
// ReachabilityFactWire for a call that actually resolved to a sink, so a
// source with no resolved sink never appears here.
func tsReachabilitySourcesReachedASink(facts []ReachabilityFact) []string {
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

func tsReachabilityHasGap(diagnostics []Diagnostic) bool {
	for _, d := range diagnostics {
		if tsReachabilityGapDiagnosticCodes[d.Code] {
			return true
		}
	}
	return false
}

func tsReachabilityCoverage(modelCoverage Coverage) Coverage {
	coverage := modelCoverage
	coverage.Complete = coverage.Complete && !tsReachabilityHasGap(coverage.Diagnostics)
	return coverage
}
