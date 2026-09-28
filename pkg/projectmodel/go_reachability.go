package projectmodel

import (
	"context"

	"go/types"
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

// callGraphIncomplete means adjacency itself is missing edges the
// underlying call-graph build could not resolve within its own budget
// (CallGraphResult.Coverage.Complete false). A BFS over an incompletely
// built adjacency can only prove "no path found within this partial
// graph", never "no path found within the full traversal" -- so every
// pair searched against it must count as truncated, not evaluated, or
// Coverage would misreport a budget-truncated call graph the same way
// it reports a genuinely complete one.

// Only append the search-level marker if neither the call-graph layer
// nor findGoReachabilitySources already recorded a
// DiagReachabilityBudgetExceeded for this same ctx/budget exhaustion
// (e.g. an already-cancelled ctx is observed at both call sites);
// otherwise the same event would be reported twice.

// reachabilitySearch is the result of one source/sink path search.
// truncatedSearch only reflects this search's own budget (MaxSearchNodes,
// ctx wall-time/cancellation), not callGraphIncomplete: the underlying
// call graph's own incompleteness diagnostic (e.g.
// DiagCallUnresolvedSyntheticWrapper) already surfaces via
// callGraph.Coverage.Diagnostics, and callGraphIncomplete already forces
// every pair to skip as truncated at reachabilityFactsForSource's call
// site. Folding it into truncatedSearch too would additionally claim a
// project_reachability_budget_exceeded that never happened.
type reachabilitySearch struct {
	facts           []ReachabilityFact
	evaluated       int
	truncatedPairs  int
	nodesVisited    int
	truncatedSearch bool
}

// reachabilityFactsForSource evaluates source against every sink using
// parents (source's own BFS parent map from bfsBudget.shortestPaths). skip is
// true when this source/sink evaluation cannot be trusted -- a budget was
// hit, ctx was cancelled, or the underlying call graph itself is
// incomplete -- in which case every pair counts as truncated rather than
// evaluated, per BuildGoReachability's callGraphIncomplete contract.

// findGoReachabilitySourcesFromLoaded walks loaded's local functions and
// returns every function whose signature is identical to
// net/http.HandlerFunc's underlying func(http.ResponseWriter, *http.Request).
// Source identification needs each function's own signature, which
// CallGraphResult's From/To strings do not carry, so this walk stays
// separate from the call-graph walk rather than growing CallFact.

type goReachabilitySourceSearch struct {
	seen        map[string]bool
	diagnostics []Diagnostic
	complete    bool
}

func (s *goReachabilitySourceSearch) collectRoot(ctx context.Context, loaded *loadedGoSnapshot, root loadedGoRoot) (stop bool) {
	if ctx.Err() != nil {
		s.complete = false
		s.diagnostics = append(s.diagnostics, Diagnostic{Code: DiagReachabilityBudgetExceeded, Path: root.dir})
		return true
	}
	if root.loadErr != nil {
		s.complete = false
		s.diagnostics = append(s.diagnostics, Diagnostic{Code: DiagReachabilitySourceLoadFailed, Path: root.dir, Message: stripTempDir(root.loadErr.Error(), loaded.tempDir)})
		return false
	}
	for _, p := range root.pkgs {
		if len(p.Errors) > 0 {
			s.complete = false
		}
	}
	handlerSig := httpHandlerFuncSignature(root.prog, root.pkgs)
	if handlerSig == nil {
		return false
	}
	for _, fn := range sortedLocalFunctions(root.prog, root.localPkgPaths) {
		if ctx.Err() != nil {
			s.complete = false
			s.diagnostics = append(s.diagnostics, Diagnostic{Code: DiagReachabilityBudgetExceeded, Path: root.dir})
			return true
		}
		if types.Identical(fn.Signature, handlerSig) {
			s.seen[fn.RelString(nil)] = true
		}
	}
	return false
}

// httpHandlerFuncSignature looks up net/http.HandlerFunc's underlying
// *types.Signature from prog or the initial packages' type-checker import
// graph, returning nil if net/http was not part of this root's build.

// containsDiagnosticCode reports whether diags already has an entry with the
// given Code.
func containsDiagnosticCode(diags []Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

// buildCallGraphAdjacency renders facts as a sorted, deduplicated adjacency
// map so bfsShortestPaths' tie-breaking never depends on facts' input
// order or Go map iteration order.

// bfsBudget is the shared node-visit counter for shortest-path walks.
// visited is not reset per source: max bounds the total number of nodes
// dequeued across the whole search.
type bfsBudget struct {
	visited int
	max     int
}

// shortestPaths runs a single breadth-first traversal from source over
// adjacency (whose neighbor lists must already be sorted), returning a
// parent map spanning every node reached before a budget or context
// deadline stopped the walk. Because neighbors are visited in sorted order
// and each node is enqueued at most once (on first discovery), the
// resulting shortest-path tree is deterministic even when multiple
// equal-length paths exist.

// reconstructReachabilityPath walks parents (as built by bfsShortestPaths)
// from sink back to source, returning the ordered source-to-sink path.

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
