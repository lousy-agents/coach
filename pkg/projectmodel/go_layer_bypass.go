package projectmodel

import (
	"context"
	"fmt"
	"io/fs"
	"runtime"
	"sort"
	"time"

	"github.com/lousy-agents/coach/pkg/projectmodel/internal/ssaload"
)

// LayerBypassOptions bounds one BuildGoLayerBypass call. Roots and Budgets
// are forwarded to the underlying BuildGoCallGraph call. MaxSearchNodes
// additionally bounds the total number of call-graph nodes visited across
// every source's BFS traversal combined; zero means unbounded.
type LayerBypassOptions struct {
	RequiredLayer  BypassLayer
	Roots          []string
	Budgets        GoBudgets
	MaxSearchNodes int
}

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

	loaded, err := loadGoSnapshot(ctx, ssaload.Loader{}, snapshot, opts.Roots, opts.Budgets)
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
	// ambiguousLayer guards against the false-positive case where an
	// unconfigured/unmatched RequiredLayer would remove nothing from
	// adjacency, silently turning this into an ordinary reachability search
	// that could misreport a genuinely compliant path as a bypass witness.
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

func searchLayerBypassWitnesses(ctx context.Context, sources, sinks []string, bypassAdjacency map[string][]string, nodePositions map[string]layerBypassNodePosition, opts LayerBypassOptions, callGraphIncomplete, dirsComplete, ambiguousLayer bool) layerBypassSearch {
	var search layerBypassSearch
	if ambiguousLayer {
		search.truncatedPairs = len(sources) * len(sinks)
		search.truncatedSearch = true
		return search
	}
	budget := bfsBudget{max: opts.MaxSearchNodes}
	for _, source := range sources {
		if ctx.Err() != nil {
			search.truncatedSearch = true
			break
		}
		parents, hitBudget := budget.shortestPaths(ctx, source, bypassAdjacency)
		if hitBudget {
			search.truncatedSearch = true
		}
		search.collectWitnesses(source, sinks, parents, nodePositions, opts.RequiredLayer.Name, hitBudget || ctx.Err() != nil || callGraphIncomplete || !dirsComplete)
	}
	if !search.truncatedSearch && ctx.Err() != nil {
		search.truncatedSearch = true
	}
	search.nodesVisited = budget.visited
	return search
}

// effectiveLayerBypassBudgets renders opts as LayerBypassResult.Coverage.Budgets,
// mirroring effectiveReachabilityBudgets exactly: the full
// EffectiveGoBudgets(opts.Budgets) vocabulary plus a "search_nodes" key for
// MaxSearchNodes.
func effectiveLayerBypassBudgets(opts LayerBypassOptions) map[string]int {
	budgets := EffectiveGoBudgets(opts.Budgets)
	budgets["search_nodes"] = opts.MaxSearchNodes
	return budgets
}
