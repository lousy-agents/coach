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
		Coverage:  model.Coverage,
	}, nil
}

func tsReachabilitySources(facts []ReachabilityFact) []string {
	seen := map[string]bool{}
	for _, f := range facts {
		seen[f.Source] = true
	}
	return mapKeysSorted(seen)
}
