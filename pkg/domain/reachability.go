package domain

import (
	"sort"
)

// ReachabilityAlgorithm identifies the pinned deterministic possible-call-
// reachability traversal. It is distinct from the call-graph algorithm:
// the two evolve independently.
const ReachabilityAlgorithm = "go-source-sink-registry@1"

// TSReachabilityAlgorithm identifies the TypeScript sidecar's own
// possible-call-reachability traversal, matching the exact string
// js/semantics/src/project-sidecar/reachability-registry.ts's
// REACHABILITY_ALGORITHM constant emits on every ReachabilityFactWire.
// Go and TypeScript must report the same algorithm-version string only
// when describing the same wire-produced facts, not two independently
// invented identifiers. It is distinct from ReachabilityAlgorithm: the
// two traversal implementations evolve independently.
const TSReachabilityAlgorithm = "ts-source-sink-registry@1"

// KindPossibleCallReachability is ReachabilityFact.Kind's fixed value.
const KindPossibleCallReachability = "possible_call_reachability"

// Stable diagnostic codes for ReachabilityResult.Coverage.Diagnostics[i].Code.
const (
	DiagReachabilityBudgetExceeded   = "project_reachability_budget_exceeded"
	DiagReachabilitySourceLoadFailed = "project_reachability_source_load_failed"
)

// ReachabilityConfidence classifies how a ReachabilityFact's Path was
// derived.
type ReachabilityConfidence string

// ReachabilityConfidenceResolvedDirect is the value produced when every hop
// in Path is a direct, statically resolved edge.
const ReachabilityConfidenceResolvedDirect ReachabilityConfidence = "resolved_direct"

// ReachabilityStep is one node in a ReachabilityFact's Path, ordered from
// source to sink.
type ReachabilityStep struct {
	NodeID string `json:"node_id"`
}

// ReachabilityFact is one possible-call-reachability observation: a source
// registry entry has a statically resolved call path to a sink registry
// entry. It is facts-only. The absence of a ReachabilityFact for a given
// source/sink pair means only "no path was found within the coverage this
// run achieved"; it is never a "safe" claim.
type ReachabilityFact struct {
	ID               string                 `json:"id"`
	Kind             string                 `json:"kind"`
	Confidence       ReachabilityConfidence `json:"confidence"`
	Source           string                 `json:"source"`
	Sink             string                 `json:"sink"`
	Path             []ReachabilityStep     `json:"path"`
	AlgorithmVersion string                 `json:"algorithm_version"`
}

// ReachabilityResult is the bounded, versioned set of ReachabilityFacts
// for a snapshot, plus the coverage of that search.
type ReachabilityResult struct {
	Facts     []ReachabilityFact
	Sources   []string
	Algorithm string
	Coverage  Coverage
}

func canonicalReachabilityFacts(in []ReachabilityFact) []ReachabilityFact {
	if len(in) == 0 {
		return in
	}
	out := append([]ReachabilityFact(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		if out[i].Sink != out[j].Sink {
			return out[i].Sink < out[j].Sink
		}
		return out[i].ID < out[j].ID
	})
	return out
}
