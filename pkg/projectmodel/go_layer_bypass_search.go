package projectmodel

import (
	"fmt"
)

// layerBypassSearch is the result of one source/sink witness search.
// truncatedSearch reflects this search's own budget (MaxSearchNodes, ctx
// wall-time/cancellation) plus the one non-budget case, ambiguousLayer,
// where the search never runs at all. It does not reflect
// callGraphIncomplete or !dirsComplete: both already carry their own
// diagnostic and already force complete to false, and both already force
// every pair to skip as truncated. Folding either into truncatedSearch
// would additionally claim a project_layer_bypass_budget_exceeded that
// never happened -- e.g. a call-graph dead end at a local-targeted
// synthetic wrapper, or a root load failure.
type layerBypassSearch struct {
	witnesses            []LayerBypassWitness
	evaluated            int
	truncatedPairs       int
	nodesVisited         int
	truncatedSearch      bool
	unclassifiedNodeSeen bool
}

func (s *layerBypassSearch) collectWitnesses(source string, sinks []string, parents map[string]string, nodePositions map[string]layerBypassNodePosition, requiredLayer string, skip bool) {
	for _, sink := range sinks {
		if skip {
			s.truncatedPairs++
			continue
		}
		s.evaluated++
		stepPath, ok := reconstructReachabilityPath(parents, source, sink)
		if !ok {
			continue
		}
		if !stepPathFullyClassified(stepPath, nodePositions) {
			s.unclassifiedNodeSeen = true
			continue
		}
		s.witnesses = append(s.witnesses, LayerBypassWitness{
			ID:               fmt.Sprintf("bypass:%s:%s->%s@%s", requiredLayer, source, sink, LayerBypassAlgorithm),
			Source:           source,
			Sink:             sink,
			RequiredLayer:    requiredLayer,
			Path:             layerBypassSteps(stepPath, nodePositions),
			Confidence:       LayerBypassConfidenceHigh,
			AlgorithmVersion: LayerBypassAlgorithm,
		})
	}
}
