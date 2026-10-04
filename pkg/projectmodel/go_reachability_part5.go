package projectmodel

import (
	"fmt"
)

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
