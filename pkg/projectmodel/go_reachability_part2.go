package projectmodel

import (
	"context"
	"fmt"

	"io/fs"
	"runtime"
	"sort"
	"time"
)

// BuildGoReachability finds possible-call-reachability facts between
// BuildGoCallGraph's direct CallFacts edges and the built-in source/sink
// registry: a source is any local function whose signature is identical to
// net/http.HandlerFunc's func(http.ResponseWriter, *http.Request) (the
// same shape evidenced by BuildGoCallGraph's framework-registration
// diagnostics, but resolved independently here since CallFact's From/To
// strings carry no signature information); a sink is any CallFact target
// matching ReachabilitySinkPatterns.
//
// For every identified source, a single deterministic BFS over the
// call-graph's adjacency (sorted per node, so tied shortest paths always
// resolve the same way) finds the shortest path to every reachable sink.
// A source/sink pair with no path within the traversal contributes no
// ReachabilityFact -- see ReachabilityFact's doc comment: that is never a
// "safe" claim, only "not found within Coverage".
//
// BuildGoReachability never returns a non-nil error for a per-root load
// failure or budget/context exhaustion; those are reported through
// Coverage.Diagnostics/Coverage.Complete, matching BuildGoCallGraph's
// fail-open-with-diagnostics contract.
//
// Coverage.Counts["memory_bytes"] is a coarse proxy, not a measurement of
// this call's own footprint: it is the delta of runtime.MemStats.TotalAlloc
// (a process-wide, monotonically increasing cumulative-allocation counter)
// taken before and after the call, so it includes GC'd churn and any
// concurrent allocation elsewhere in the process during the call. Treat it
// as an upper bound, not a peak or exclusive figure.
func BuildGoReachability(ctx context.Context, snapshot fs.FS, opts ReachabilityOptions) (ReachabilityResult, error) {
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
		return ReachabilityResult{}, fmt.Errorf("projectmodel: building call graph for reachability: %w", err)
	}
	defer loaded.cleanup()

	callGraph := buildGoCallGraphFromLoaded(ctx, loaded, CallGraphOptions{Roots: opts.Roots, Budgets: opts.Budgets})
	sources, sourcesComplete, sourceDiagnostics := findGoReachabilitySourcesFromLoaded(ctx, loaded)
	adjacency := buildCallGraphAdjacency(callGraph.CallFacts)

	sinks := append([]string(nil), ReachabilitySinkPatterns...)
	sort.Strings(sinks)

	callGraphIncomplete := !callGraph.Coverage.Complete

	search := searchReachabilityFacts(ctx, sources, sinks, adjacency, opts.MaxSearchNodes, callGraphIncomplete)

	runtime.ReadMemStats(&memAfter)
	memDelta := int64(memAfter.TotalAlloc) - int64(memBefore.TotalAlloc)
	if memDelta < 0 {
		memDelta = 0
	}

	diagnostics := append([]Diagnostic{}, callGraph.Coverage.Diagnostics...)
	diagnostics = append(diagnostics, sourceDiagnostics...)

	if search.truncatedSearch && !containsDiagnosticCode(diagnostics, DiagReachabilityBudgetExceeded) {
		diagnostics = append(diagnostics, Diagnostic{Code: DiagReachabilityBudgetExceeded})
	}

	complete := callGraph.Coverage.Complete && sourcesComplete && !search.truncatedSearch

	sort.Slice(search.facts, func(i, j int) bool {
		if search.facts[i].Source != search.facts[j].Source {
			return search.facts[i].Source < search.facts[j].Source
		}
		return search.facts[i].Sink < search.facts[j].Sink
	})

	return ReachabilityResult{
		Facts:     search.facts,
		Sources:   sources,
		Algorithm: ReachabilityAlgorithm,
		Coverage: canonicalCoverage(Coverage{
			Phase:    "go_reachability",
			Complete: complete,
			Counts: map[string]int{
				"sources_identified":               len(sources),
				"sinks_pinned":                     len(sinks),
				"source_sink_pairs_total":          len(sources) * len(sinks),
				"source_sink_pairs_evaluated":      search.evaluated,
				"source_sink_pairs_truncated":      search.truncatedPairs,
				"reachable_pairs":                  len(search.facts),
				"underlying_call_sites_seen":       callGraph.Coverage.Counts["call_sites_seen"],
				"underlying_unresolved_call_sites": unresolvedCallSiteCount(callGraph.Coverage.Counts),
				"ssa_programs_built":               loaded.programsBuilt(),
				"runtime_ms":                       int(time.Since(start) / time.Millisecond),
				"memory_bytes":                     int(memDelta),
			},
			Budgets:     effectiveReachabilityBudgets(opts),
			Diagnostics: diagnostics,
		}),
	}, nil
}
func searchReachabilityFacts(ctx context.Context, sources, sinks []string, adjacency map[string][]string, maxSearchNodes int, callGraphIncomplete bool) reachabilitySearch {
	var search reachabilitySearch
	budget := bfsBudget{max: maxSearchNodes}
	for _, source := range sources {
		if ctx.Err() != nil {
			search.truncatedSearch = true
			break
		}
		parents, hitBudget := budget.shortestPaths(ctx, source, adjacency)
		if hitBudget {
			search.truncatedSearch = true
		}
		sourceFacts, sourceEvaluated, sourceTruncated := reachabilityFactsForSource(source, sinks, parents, hitBudget || ctx.Err() != nil || callGraphIncomplete)
		search.facts = append(search.facts, sourceFacts...)
		search.evaluated += sourceEvaluated
		search.truncatedPairs += sourceTruncated
	}
	if !search.truncatedSearch && ctx.Err() != nil {
		search.truncatedSearch = true
	}
	search.nodesVisited = budget.visited
	return search
}
