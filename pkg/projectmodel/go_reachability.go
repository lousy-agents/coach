package projectmodel

import (
	"context"
	"fmt"
	"go/types"
	"io/fs"
	"runtime"
	"sort"
	"time"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// ReachabilityAlgorithm identifies the pinned deterministic possible-call-
// reachability traversal BuildGoReachability uses to find a path from a
// registry source to a registry sink over a CallGraphResult's CallFacts
// edges. It is distinct from CallGraphAlgorithm: the two evolve
// independently (the call-graph resolution method vs. the source/sink
// registry and search strategy layered on top of it).
const ReachabilityAlgorithm = "go-source-sink-registry@1"

// KindPossibleCallReachability is ReachabilityFact.Kind's fixed value. It
// matches the Kind string pkg/codesignal's ProjectFact expects for this
// observation once a later task maps ReachabilityFact onto ProjectFact.
const KindPossibleCallReachability = "possible_call_reachability"

// Stable diagnostic codes for ReachabilityResult.Coverage.Diagnostics[i].Code.
const (
	DiagReachabilityBudgetExceeded   = "project_reachability_budget_exceeded"
	DiagReachabilitySourceLoadFailed = "project_reachability_source_load_failed"
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

// ReachabilityConfidence classifies how a ReachabilityFact's Path was
// derived.
type ReachabilityConfidence string

// ReachabilityConfidenceResolvedDirect is currently the only value
// BuildGoReachability produces: every hop in Path is a direct, statically
// resolved CallFact edge (BuildGoCallGraph never emits a CallFact for an
// unresolved call site), so the whole path is provable, not a guess through
// an interface, function-value, reflection, framework-registration, or
// local-targeted synthetic-wrapper boundary.
const ReachabilityConfidenceResolvedDirect ReachabilityConfidence = "resolved_direct"

// ReachabilityStep is one node in a ReachabilityFact's Path, ordered from
// source to sink.
type ReachabilityStep struct {
	NodeID string `json:"node_id"`
}

// ReachabilityFact is one possible-call-reachability observation: a source
// registry entry has a statically resolved call path to a sink registry
// entry. It is facts-only -- deliberately carrying no severity, lifecycle,
// or active-finding field of any kind. The absence of a ReachabilityFact
// for a given source/sink pair means only "no path was found within the
// coverage this run achieved" (see ReachabilityResult.Coverage); it is
// never a "safe"/"verified absent" claim.
type ReachabilityFact struct {
	ID               string                 `json:"id"`
	Kind             string                 `json:"kind"`
	Confidence       ReachabilityConfidence `json:"confidence"`
	Source           string                 `json:"source"`
	Sink             string                 `json:"sink"`
	Path             []ReachabilityStep     `json:"path"`
	AlgorithmVersion string                 `json:"algorithm_version"`
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

// ReachabilityResult is the bounded, versioned set of ReachabilityFacts
// BuildGoReachability produces for a Snapshot, plus the coverage
// measurement (path coverage, unresolved ratio, runtime, memory,
// truncation) needed to judge how much of the source x sink space was
// actually searched.
type ReachabilityResult struct {
	// Facts holds one entry per source/sink pair with a statically resolved
	// path found within budget. Sorted by Source then Sink.
	Facts []ReachabilityFact
	// Sources holds every registry source function identified in the
	// snapshot, sorted.
	Sources   []string
	Algorithm string
	Coverage  Coverage
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

func searchReachabilityFacts(ctx context.Context, sources, sinks []string, adjacency map[string][]string, maxSearchNodes int, callGraphIncomplete bool) reachabilitySearch {
	var search reachabilitySearch
	for _, source := range sources {
		if ctx.Err() != nil {
			search.truncatedSearch = true
			break
		}
		parents, hitBudget := bfsShortestPaths(ctx, source, adjacency, maxSearchNodes, &search.nodesVisited)
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
	return search
}

// reachabilityFactsForSource evaluates source against every sink using
// parents (source's own BFS parent map from bfsShortestPaths). skip is
// true when this source/sink evaluation cannot be trusted -- a budget was
// hit, ctx was cancelled, or the underlying call graph itself is
// incomplete -- in which case every pair counts as truncated rather than
// evaluated, per BuildGoReachability's callGraphIncomplete contract.
func reachabilityFactsForSource(source string, sinks []string, parents map[string]string, skip bool) (facts []ReachabilityFact, evaluated, truncated int) {
	for _, sink := range sinks {
		if skip {
			truncated++
			continue
		}
		evaluated++
		path, ok := reconstructReachabilityPath(parents, source, sink)
		if !ok {
			continue
		}
		facts = append(facts, ReachabilityFact{
			ID:               fmt.Sprintf("reach:%s->%s@%s", source, sink, ReachabilityAlgorithm),
			Kind:             KindPossibleCallReachability,
			Confidence:       ReachabilityConfidenceResolvedDirect,
			Source:           source,
			Sink:             sink,
			Path:             path,
			AlgorithmVersion: ReachabilityAlgorithm,
		})
	}
	return facts, evaluated, truncated
}

// findGoReachabilitySourcesFromLoaded walks loaded's local functions and
// returns every function whose signature is identical to
// net/http.HandlerFunc's underlying func(http.ResponseWriter, *http.Request).
// Source identification needs each function's own signature, which
// CallGraphResult's From/To strings do not carry, so this walk stays
// separate from the call-graph walk rather than growing CallFact.
func findGoReachabilitySourcesFromLoaded(ctx context.Context, loaded *loadedGoSnapshot) ([]string, bool, []Diagnostic) {
	if ctx.Err() != nil {
		return nil, false, []Diagnostic{{Code: DiagReachabilityBudgetExceeded}}
	}

	complete := loaded.discovery.Complete
	var diagnostics []Diagnostic
	seen := map[string]bool{}

	for _, root := range loaded.roots {
		if ctx.Err() != nil {
			complete = false
			diagnostics = append(diagnostics, Diagnostic{Code: DiagReachabilityBudgetExceeded, Path: root.dir})
			break
		}
		if root.loadErr != nil {
			complete = false
			diagnostics = append(diagnostics, Diagnostic{Code: DiagReachabilitySourceLoadFailed, Path: root.dir, Message: stripTempDir(root.loadErr.Error(), loaded.tempDir)})
			continue
		}
		for _, p := range root.pkgs {
			if len(p.Errors) > 0 {
				complete = false
			}
		}

		handlerSig := httpHandlerFuncSignature(root.prog, root.pkgs)
		if handlerSig == nil {
			continue
		}
		for _, fn := range sortedLocalFunctions(root.prog, root.localPkgPaths) {
			if ctx.Err() != nil {
				complete = false
				diagnostics = append(diagnostics, Diagnostic{Code: DiagReachabilityBudgetExceeded, Path: root.dir})
				break
			}
			if types.Identical(fn.Signature, handlerSig) {
				seen[fn.RelString(nil)] = true
			}
		}
	}

	return mapKeysSorted(seen), complete, diagnostics
}

// httpHandlerFuncSignature looks up net/http.HandlerFunc's underlying
// *types.Signature from prog or the initial packages' type-checker import
// graph, returning nil if net/http was not part of this root's build.
func httpHandlerFuncSignature(prog *ssa.Program, pkgs []*packages.Package) *types.Signature {
	tp := typesPackageByPath(prog, pkgs, "net/http")
	if tp == nil {
		return nil
	}
	obj := tp.Scope().Lookup("HandlerFunc")
	if obj == nil {
		return nil
	}
	sig, ok := obj.Type().Underlying().(*types.Signature)
	if !ok {
		return nil
	}
	return sig
}

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
func buildCallGraphAdjacency(facts []CallFact) map[string][]string {
	tmp := map[string]map[string]bool{}
	for _, f := range facts {
		if tmp[f.From] == nil {
			tmp[f.From] = map[string]bool{}
		}
		tmp[f.From][f.To] = true
	}
	adjacency := make(map[string][]string, len(tmp))
	for from, tos := range tmp {
		adjacency[from] = mapKeysSorted(tos)
	}
	return adjacency
}

// bfsShortestPaths runs a single breadth-first traversal from source over
// adjacency (whose neighbor lists must already be sorted), returning a
// parent map spanning every node reached before a budget or context
// deadline stopped the walk. Because neighbors are visited in sorted order
// and each node is enqueued at most once (on first discovery), the
// resulting shortest-path tree is deterministic even when multiple
// equal-length paths exist. maxNodes bounds the total number of nodes
// dequeued across the whole BuildGoReachability call (visited is shared via
// the nodesVisited counter, not reset per source).
func bfsShortestPaths(ctx context.Context, source string, adjacency map[string][]string, maxNodes int, nodesVisited *int) (map[string]string, bool) {
	parents := map[string]string{source: ""}
	visited := map[string]bool{source: true}
	queue := []string{source}

	for len(queue) > 0 {
		if ctx.Err() != nil {
			return parents, true
		}
		if maxNodes > 0 && *nodesVisited >= maxNodes {
			return parents, true
		}
		node := queue[0]
		queue = queue[1:]
		*nodesVisited++

		for _, next := range adjacency[node] {
			if visited[next] {
				continue
			}
			visited[next] = true
			parents[next] = node
			queue = append(queue, next)
		}
	}
	return parents, false
}

// reconstructReachabilityPath walks parents (as built by bfsShortestPaths)
// from sink back to source, returning the ordered source-to-sink path.
func reconstructReachabilityPath(parents map[string]string, source, sink string) ([]ReachabilityStep, bool) {
	if _, ok := parents[sink]; !ok {
		return nil, false
	}
	var nodes []string
	for cur := sink; ; {
		nodes = append(nodes, cur)
		if cur == source {
			break
		}
		cur = parents[cur]
	}
	steps := make([]ReachabilityStep, len(nodes))
	for i, n := range nodes {
		steps[len(nodes)-1-i] = ReachabilityStep{NodeID: n}
	}
	return steps, true
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
