package projectmodel

import (
	"context"
	"fmt"
)

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

// reachabilityFactsForSource evaluates source against every sink using
// parents (source's own BFS parent map from bfsBudget.shortestPaths). skip is
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
