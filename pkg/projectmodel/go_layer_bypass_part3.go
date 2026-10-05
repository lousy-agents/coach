package projectmodel

import (
	"context"
	"fmt"

	"io/fs"

	"runtime"
	"sort"
	"time"
)

// BuildGoLayerBypass finds layer-bypass witnesses: a source/sink pair (the
// same handler-shaped source and pinned database-sink registry
// BuildGoReachability uses) with a statically resolved CallFact path that
// survives deleting every call-graph node whose owning package directory
// falls under opts.RequiredLayer's prefixes. A surviving path is, by
// construction, a route from source to sink that never passes through the
// required layer -- the witness IS the proof, and it is emitted whenever
// step finds one, regardless of whether a separate compliant path (one that
// does pass through the required layer) also exists elsewhere in the graph.
//
// A witness is only ever emitted at LayerBypassConfidenceHigh, and only when
// opts.RequiredLayer is unambiguous (its Prefixes are non-empty and match at
// least one local package in the snapshot -- see
// DiagLayerBypassAmbiguousLayer) and the underlying call graph, source
// identification, and node-classification walk all completed within budget
// for the pair involved. Any of those conditions failing suppresses the
// witness for that pair entirely rather than downgrading its confidence;
// see LayerBypassResult.Coverage for what was and was not evaluated.
//
// BuildGoLayerBypass never returns a non-nil error for a per-root load
// failure or budget/context exhaustion; those are reported through
// Coverage.Diagnostics/Coverage.Complete, matching BuildGoReachability's
// fail-open-with-diagnostics contract.
func BuildGoLayerBypass(ctx context.Context, snapshot fs.FS, opts LayerBypassOptions) (LayerBypassResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.Budgets.WallTime > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Budgets.WallTime)
		defer cancel()
	}

	start := time.Now()
	var memBefore, memAfter runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	loaded, err := loadGoSnapshot(ctx, snapshot, opts.Roots, opts.Budgets)
	if err != nil {
		return LayerBypassResult{}, fmt.Errorf("projectmodel: building call graph for layer bypass: %w", err)
	}
	defer loaded.cleanup()

	callGraph := buildGoCallGraphFromLoaded(ctx, loaded, CallGraphOptions{Roots: opts.Roots, Budgets: opts.Budgets})
	sources, sourcesComplete, rawSourceDiagnostics := findGoReachabilitySourcesFromLoaded(ctx, loaded)
	sourceDiagnostics := remapDiagnosticCodes(rawSourceDiagnostics, map[string]string{
		DiagReachabilityBudgetExceeded:   DiagLayerBypassBudgetExceeded,
		DiagReachabilitySourceLoadFailed: DiagLayerBypassSourceLoadFailed,
	})

	nodePositions, dirsComplete, dirDiagnostics := layerBypassNodePackageDirsFromLoaded(ctx, loaded)

	adjacency := buildCallGraphAdjacency(callGraph.CallFacts)
	sinks := append([]string(nil), ReachabilitySinkPatterns...)
	sort.Strings(sinks)

	callGraphIncomplete := !callGraph.Coverage.Complete

	requiredLayerNodes := requiredLayerNodeSet(opts.RequiredLayer, nodePositions)

	ambiguousLayer := len(opts.RequiredLayer.Prefixes) == 0 || len(requiredLayerNodes) == 0

	bypassAdjacency := adjacency
	if !ambiguousLayer {
		bypassAdjacency = removeLayerNodesFromAdjacency(adjacency, requiredLayerNodes)
	}

	search := searchLayerBypassWitnesses(ctx, sources, sinks, bypassAdjacency, nodePositions, opts, callGraphIncomplete, dirsComplete, ambiguousLayer)

	runtime.ReadMemStats(&memAfter)
	memDelta := int64(memAfter.TotalAlloc) - int64(memBefore.TotalAlloc)
	if memDelta < 0 {
		memDelta = 0
	}

	return assembleLayerBypassResult(loaded, callGraph, sources, sinks, requiredLayerNodes, search, sourceDiagnostics, dirDiagnostics, sourcesComplete, dirsComplete, ambiguousLayer, opts, start, memDelta), nil
}
func assembleLayerBypassResult(loaded *loadedGoSnapshot, callGraph CallGraphResult, sources, sinks []string, requiredLayerNodes map[string]bool, search layerBypassSearch, sourceDiagnostics, dirDiagnostics []Diagnostic, sourcesComplete, dirsComplete, ambiguousLayer bool, opts LayerBypassOptions, start time.Time, memDelta int64) LayerBypassResult {
	diagnostics := append([]Diagnostic{}, callGraph.Coverage.Diagnostics...)
	diagnostics = append(diagnostics, sourceDiagnostics...)
	diagnostics = append(diagnostics, dirDiagnostics...)
	if ambiguousLayer {
		diagnostics = append(diagnostics, Diagnostic{Code: DiagLayerBypassAmbiguousLayer})
	} else if search.unclassifiedNodeSeen && !containsDiagnosticCode(diagnostics, DiagLayerBypassAmbiguousLayer) {
		diagnostics = append(diagnostics, Diagnostic{Code: DiagLayerBypassAmbiguousLayer})
	}
	if search.truncatedSearch && !containsDiagnosticCode(diagnostics, DiagLayerBypassBudgetExceeded) {
		diagnostics = append(diagnostics, Diagnostic{Code: DiagLayerBypassBudgetExceeded})
	}

	sort.Slice(search.witnesses, func(i, j int) bool {
		if search.witnesses[i].Source != search.witnesses[j].Source {
			return search.witnesses[i].Source < search.witnesses[j].Source
		}
		return search.witnesses[i].Sink < search.witnesses[j].Sink
	})

	return LayerBypassResult{
		Witnesses: search.witnesses,
		Sources:   sources,
		Algorithm: LayerBypassAlgorithm,
		Coverage: canonicalCoverage(Coverage{
			Phase:    "go_layer_bypass",
			Complete: callGraph.Coverage.Complete && sourcesComplete && dirsComplete && !search.truncatedSearch,
			Counts: map[string]int{
				"sources_identified":               len(sources),
				"sinks_pinned":                     len(sinks),
				"source_sink_pairs_total":          len(sources) * len(sinks),
				"source_sink_pairs_evaluated":      search.evaluated,
				"source_sink_pairs_truncated":      search.truncatedPairs,
				"witnesses_found":                  len(search.witnesses),
				"required_layer_nodes_matched":     len(requiredLayerNodes),
				"underlying_call_sites_seen":       callGraph.Coverage.Counts["call_sites_seen"],
				"underlying_unresolved_call_sites": unresolvedCallSiteCount(callGraph.Coverage.Counts),
				"ssa_programs_built":               loaded.programsBuilt(),
				"search_nodes_visited":             search.nodesVisited,
				"runtime_ms":                       int(time.Since(start) / time.Millisecond),
				"memory_bytes":                     int(memDelta),
			},
			Budgets:     effectiveLayerBypassBudgets(opts),
			Diagnostics: diagnostics,
		}),
	}
}
