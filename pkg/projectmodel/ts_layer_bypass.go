package projectmodel

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// TSLayerBypassAlgorithm identifies BuildTypeScriptLayerBypass's traversal:
// the same CallFact adjacency and required-layer node-removal strategy
// BuildGoLayerBypass uses, applied over the TS sidecar's own CallFacts/
// ReachabilityFacts instead of an SSA-resolved call graph. It is distinct
// from Go's LayerBypassAlgorithm ("go-layer-bypass-registry@1"): the two
// traversals evolve independently.
const TSLayerBypassAlgorithm = "ts-layer-bypass-registry@1"

// BuildTypeScriptLayerBypass finds layer-bypass witnesses over the same TS
// sidecar round trip BuildTypeScriptReachability makes: it calls
// BuildTypeScriptModelViaSidecar once, then removes every CallFact node
// whose "file:<repo-relative path>#<name>" node ID (see
// reachability.ts's functionSourceId) resolves a declaration
// directory under requiredLayer's Prefixes, and runs the same deterministic
// BFS BuildGoLayerBypass uses over the reduced adjacency for every source x
// sink pair. A surviving path never passed through requiredLayer, exactly
// as on the Go side -- see removeLayerNodesFromAdjacency/bfsShortestPaths
// in go_reachability.go/go_layer_bypass.go, both reused unchanged here.
//
// Sources and sinks are both derived from Model.ReachabilityFacts (the
// deduplicated, sorted set of Source/Sink values respectively) -- the same
// narrower TS convention BuildTypeScriptReachability already established
// for Sources, since the TS wire protocol has no independent source/sink
// identification walk the way Go's findGoReachabilitySourcesFromLoaded and
// pinned ReachabilitySinkPatterns do.
//
// A witness is only ever emitted at LayerBypassConfidenceHigh, and only when
// requiredLayer is unambiguous (non-empty Prefixes matching at least one
// file in the snapshot's own file inventory) and that specific source's BFS
// completed within budget (see the per-source hitBudget/ctx.Err() check
// below) and every node on the witness's own path resolved a classifiable
// position (stepPathFullyClassified). Unlike BuildGoLayerBypass,
// Model.Coverage.Complete being false does NOT by itself suppress a pair:
// the TS sidecar's single combined Coverage folds in
// ts_reachability_local_call_not_followed_gap, which fires for the routine
// case of a handler delegating one hop into a helper/service function this
// depth-1 walk does not itself follow -- the ordinary shape of layered code,
// not a rare failure. That diagnostic reflects incompleteness in a
// *different* source's own reachability picture (fewer edges recorded from
// it, never a fabricated one), so it must not suppress an unrelated,
// already fully-resolved witness. model.Coverage.Complete still gates the
// aggregate LayerBypassResult.Coverage.Complete, so the incompleteness is
// reported honestly rather than erased -- see EvaluateGoLayerBypass's doc
// comment (rule_layer_bypass.go) for how a caller must fold that into a
// witness's Lifecycle rather than treat the witness's presence as suspect.
// BuildTypeScriptLayerBypass never returns a non-nil error for a sidecar
// transport/analysis failure; that is reported through
// Coverage.Diagnostics/Coverage.Complete instead, matching
// BuildTypeScriptModelViaSidecar's fail-open-with-diagnostics contract.
func BuildTypeScriptLayerBypass(ctx context.Context, snapshot fs.FS, meta SnapshotMeta, opts TSSidecarOptions, requiredLayer BypassLayer) (LayerBypassResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	model, err := BuildTypeScriptModelViaSidecar(ctx, snapshot, meta, opts)
	if err != nil {
		return LayerBypassResult{}, err
	}

	sources := tsReachabilitySources(model.ReachabilityFacts)
	sinks := tsLayerBypassSinks(model.ReachabilityFacts)
	adjacency := buildCallGraphAdjacency(model.CallFacts)
	nodePositions := tsLayerBypassNodePositions(model.CallFacts)

	requiredLayerNodes := map[string]bool{}
	for node, pos := range nodePositions {
		if layerBypassContainsDir(requiredLayer, pos.Dir) {
			requiredLayerNodes[node] = true
		}
	}
	// ambiguousLayer mirrors BuildGoLayerBypass's own guard: an
	// unconfigured/unmatched requiredLayer would remove nothing from
	// adjacency, silently turning this into an ordinary reachability search
	// that could misreport a genuinely compliant path as a bypass witness.
	//
	// The match itself is decided from model.Files, not from
	// requiredLayerNodes/CallFacts: the real sidecar's call graph only ever
	// contains route-handler-to-sink edges (see
	// js/semantics/src/project-sidecar/reachability.ts's depth-1 walk), so a
	// required layer with no route handler of its own -- the exact shape a
	// genuine layer bypass produces -- would never appear as a CallFact
	// endpoint even though real files live there. model.Files is populated
	// from the snapshot's own collected file list independent of the
	// sidecar's response (see tsFileFactsFromCollected), so it reflects what
	// is actually on disk regardless of what the call graph happened to
	// touch.
	ambiguousLayer := len(requiredLayer.Prefixes) == 0 || !tsLayerBypassLayerMatchesFiles(model.Files, requiredLayer)

	bypassAdjacency := adjacency
	if !ambiguousLayer {
		bypassAdjacency = removeLayerNodesFromAdjacency(adjacency, requiredLayerNodes)
	}

	var witnesses []LayerBypassWitness
	evaluated, truncatedPairs, nodesVisited := 0, 0, 0
	truncatedSearch := false
	unclassifiedNodeSeen := false

	if ambiguousLayer {
		truncatedPairs = len(sources) * len(sinks)
		truncatedSearch = true
	} else {
		for _, source := range sources {
			if ctx.Err() != nil {
				truncatedSearch = true
				break
			}
			parents, hitBudget := bfsShortestPaths(ctx, source, bypassAdjacency, 0, &nodesVisited)
			if hitBudget {
				truncatedSearch = true
			}
			skip := hitBudget || ctx.Err() != nil
			for _, sink := range sinks {
				if skip {
					truncatedPairs++
					continue
				}
				evaluated++
				stepPath, ok := reconstructReachabilityPath(parents, source, sink)
				if !ok {
					continue
				}
				if !stepPathFullyClassified(stepPath, nodePositions) {
					unclassifiedNodeSeen = true
					continue
				}
				witnesses = append(witnesses, LayerBypassWitness{
					ID:               fmt.Sprintf("bypass:%s:%s->%s@%s", requiredLayer.Name, source, sink, TSLayerBypassAlgorithm),
					Source:           source,
					Sink:             sink,
					RequiredLayer:    requiredLayer.Name,
					Path:             layerBypassSteps(stepPath, nodePositions),
					Confidence:       LayerBypassConfidenceHigh,
					AlgorithmVersion: TSLayerBypassAlgorithm,
				})
			}
		}
	}
	if !truncatedSearch && ctx.Err() != nil {
		truncatedSearch = true
	}

	diagnostics := append([]Diagnostic{}, model.Coverage.Diagnostics...)
	if ambiguousLayer {
		diagnostics = append(diagnostics, Diagnostic{Code: DiagLayerBypassAmbiguousLayer})
	} else if unclassifiedNodeSeen && !containsDiagnosticCode(diagnostics, DiagLayerBypassAmbiguousLayer) {
		diagnostics = append(diagnostics, Diagnostic{Code: DiagLayerBypassAmbiguousLayer})
	}
	if truncatedSearch && !containsDiagnosticCode(diagnostics, DiagLayerBypassBudgetExceeded) {
		diagnostics = append(diagnostics, Diagnostic{Code: DiagLayerBypassBudgetExceeded})
	}

	complete := model.Coverage.Complete && !truncatedSearch

	sort.Slice(witnesses, func(i, j int) bool {
		if witnesses[i].Source != witnesses[j].Source {
			return witnesses[i].Source < witnesses[j].Source
		}
		return witnesses[i].Sink < witnesses[j].Sink
	})

	return LayerBypassResult{
		Witnesses: witnesses,
		Sources:   sources,
		Algorithm: TSLayerBypassAlgorithm,
		Coverage: canonicalCoverage(Coverage{
			Phase:    "ts_layer_bypass",
			Complete: complete,
			Counts: map[string]int{
				"sources_identified":           len(sources),
				"sinks_identified":             len(sinks),
				"source_sink_pairs_total":      len(sources) * len(sinks),
				"source_sink_pairs_evaluated":  evaluated,
				"source_sink_pairs_truncated":  truncatedPairs,
				"witnesses_found":              len(witnesses),
				"required_layer_nodes_matched": len(requiredLayerNodes),
				"search_nodes_visited":         nodesVisited,
			},
			Diagnostics: diagnostics,
		}),
	}, nil
}

// tsLayerBypassLayerMatchesFiles reports whether any file in files falls
// under layer's Prefixes, reusing layerBypassContainsDir's own prefix-match
// rule.
func tsLayerBypassLayerMatchesFiles(files []File, layer BypassLayer) bool {
	for _, f := range files {
		if layerBypassContainsDir(layer, path.Dir(f.Path)) {
			return true
		}
	}
	return false
}

// tsLayerBypassSinks returns the deduplicated, sorted set of facts' Sink
// values -- the TS analog of Go's pinned ReachabilitySinkPatterns, which
// has no equivalent here since the TS sidecar's own sink registry
// (reachability-registry.ts's REACHABILITY_SINK_CLASSES) is not exposed to
// this package; only the sinks a resolved fact actually reached are known.
func tsLayerBypassSinks(facts []ReachabilityFact) []string {
	seen := map[string]bool{}
	for _, f := range facts {
		seen[f.Sink] = true
	}
	return mapKeysSorted(seen)
}

// tsLayerBypassNodePositions parses every "file:"-addressed CallFact
// endpoint (From or To) into its declaration directory/file, reusing Go's
// shared layerBypassNodePosition/layerBypassContainsDir/
// stepPathFullyClassified/layerBypassSteps classification machinery
// unchanged. Line is always left zero: the TS sidecar's call-graph walk
// threads no per-function declaration line into CallFact, unlike Go's
// SSA-resolved fnPosition, so no genuine line is available to report --
// mirroring how Go itself leaves the sink's Line zero for the analogous
// "nothing to resolve" reason.
func tsLayerBypassNodePositions(facts []CallFact) map[string]layerBypassNodePosition {
	positions := map[string]layerBypassNodePosition{}
	for _, f := range facts {
		for _, nodeID := range [2]string{f.From, f.To} {
			if _, ok := positions[nodeID]; ok {
				continue
			}
			if pos, ok := tsLayerBypassNodePosition(nodeID); ok {
				positions[nodeID] = pos
			}
		}
	}
	return positions
}

// tsLayerBypassNodePosition parses a TS call-graph node ID of the
// "file:<repo-relative path>#<name>" shape (reachability.ts's
// functionSourceId) into its declaration directory and file path. It
// reports false for a synthetic sink node ID (e.g. "(PrismaClient).findMany"),
// which carries no file path at all -- the TS analog of Go's always-
// unclassified stdlib sink (see LayerBypassStep's doc comment).
func tsLayerBypassNodePosition(nodeID string) (layerBypassNodePosition, bool) {
	rest, ok := strings.CutPrefix(nodeID, "file:")
	if !ok {
		return layerBypassNodePosition{}, false
	}
	filePath, _, ok := strings.Cut(rest, "#")
	if !ok || filePath == "" {
		return layerBypassNodePosition{}, false
	}
	return layerBypassNodePosition{Dir: path.Dir(filePath), File: filePath}, true
}
