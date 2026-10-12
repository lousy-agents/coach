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

// ReachabilitySinkPatterns is the pinned, deterministic registry of
// database-access-shaped callees possible-call-reachability treats as
// sinks. This is registry policy, not raw ProjectModel data: it is not
// derived from a snapshot, and extending it is a deliberate versioning
// decision (bump ReachabilityAlgorithm alongside any change here).
// IDs use ssa.Function.RelString(nil) form, matching CallFact.To.
var ReachabilitySinkPatterns = []string{
	"(*database/sql.DB).Exec",
	"(*database/sql.DB).ExecContext",
	"(*database/sql.DB).Query",
	"(*database/sql.DB).QueryContext",
}

// ReachabilityOptions bounds one BuildGoReachability call. Roots and
// Budgets are forwarded to the underlying BuildGoCallGraph call.
// MaxSearchNodes additionally bounds the total number of call-graph nodes
// visited across every source's BFS traversal combined; zero means
// unbounded.
type ReachabilityOptions struct {
	Roots          []string
	Budgets        GoBudgets
	MaxSearchNodes int
}

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

	loaded, err := loadGoSnapshot(ctx, ssaload.Loader{}, snapshot, opts.Roots, opts.Budgets)
	if err != nil {
		return ReachabilityResult{}, fmt.Errorf("projectmodel: building call graph for reachability: %w", err)
	}
	defer loaded.cleanup()

	callGraph := buildGoCallGraphFromLoaded(ctx, loaded, CallGraphOptions{Roots: opts.Roots, Budgets: opts.Budgets})
	sources, sourcesComplete, sourceDiagnostics := findGoReachabilitySourcesFromLoaded(ctx, loaded)
	adjacency := buildCallGraphAdjacency(callGraph.CallFacts)

	sinks := append([]string(nil), ReachabilitySinkPatterns...)
	sort.Strings(sinks)

	// callGraphIncomplete means adjacency itself is missing edges the
	// underlying call-graph build could not resolve within its own budget
	// (CallGraphResult.Coverage.Complete false). A BFS over an incompletely
	// built adjacency can only prove "no path found within this partial
	// graph", never "no path found within the full traversal" -- so every
	// pair searched against it must count as truncated, not evaluated, or
	// Coverage would misreport a budget-truncated call graph the same way
	// it reports a genuinely complete one.
	callGraphIncomplete := !callGraph.Coverage.Complete

	search := searchReachabilityFacts(ctx, sources, sinks, adjacency, opts.MaxSearchNodes, callGraphIncomplete)

	runtime.ReadMemStats(&memAfter)
	memDelta := int64(memAfter.TotalAlloc) - int64(memBefore.TotalAlloc)
	if memDelta < 0 {
		memDelta = 0
	}

	diagnostics := append([]Diagnostic{}, callGraph.Coverage.Diagnostics...)
	diagnostics = append(diagnostics, sourceDiagnostics...)
	// Only append the search-level marker if neither the call-graph layer
	// nor findGoReachabilitySources already recorded a
	// DiagReachabilityBudgetExceeded for this same ctx/budget exhaustion
	// (e.g. an already-cancelled ctx is observed at both call sites);
	// otherwise the same event would be reported twice.
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

// effectiveReachabilityBudgets renders opts as
// ReachabilityResult.Coverage.Budgets: the full EffectiveGoBudgets(opts.Budgets)
// vocabulary (the same budgets forwarded to the underlying BuildGoCallGraph
// call, so whichever one actually truncated the run stays visible here, not
// just as a diagnostic -- except a sub-millisecond WallTime, which
// EffectiveGoBudgets truncates to 0 via integer division and so reports
// indistinguishably from "unbounded") plus a "search_nodes" key for
// MaxSearchNodes, which has no analog in GoBudgets.
func effectiveReachabilityBudgets(opts ReachabilityOptions) map[string]int {
	budgets := EffectiveGoBudgets(opts.Budgets)
	budgets["search_nodes"] = opts.MaxSearchNodes
	return budgets
}
