package projectmodel

import (
	"context"
	"fmt"
)

type tsLayerBypassSearchResult struct {
	witnesses            []LayerBypassWitness
	evaluated            int
	truncatedPairs       int
	nodesVisited         int
	truncatedSearch      bool
	unclassifiedNodeSeen bool
}

func tsLayerBypassRunSearch(ctx context.Context, sources, sinks []string, adjacency map[string][]string, nodePositions map[string]layerBypassNodePosition, requiredLayer BypassLayer, ambiguousLayer bool) tsLayerBypassSearchResult {
	var result tsLayerBypassSearchResult

	if ambiguousLayer {
		result.truncatedPairs = len(sources) * len(sinks)
		result.truncatedSearch = true
	} else {
		result.searchSources(ctx, sources, sinks, adjacency, nodePositions, requiredLayer)
	}
	if !result.truncatedSearch && ctx.Err() != nil {
		result.truncatedSearch = true
	}
	return result
}

func (result *tsLayerBypassSearchResult) searchSources(ctx context.Context, sources, sinks []string, adjacency map[string][]string, nodePositions map[string]layerBypassNodePosition, requiredLayer BypassLayer) {
	budget := &bfsBudget{}
	for _, source := range sources {
		if ctx.Err() != nil {
			result.truncatedSearch = true
			break
		}
		sourceResult := tsLayerBypassSearchFromSource(ctx, source, sinks, adjacency, nodePositions, requiredLayer, budget)
		result.truncatedSearch = result.truncatedSearch || sourceResult.truncatedSearch
		result.truncatedPairs += sourceResult.truncatedPairs
		result.evaluated += sourceResult.evaluated
		result.unclassifiedNodeSeen = result.unclassifiedNodeSeen || sourceResult.unclassifiedNodeSeen
		result.witnesses = append(result.witnesses, sourceResult.witnesses...)
	}
	result.nodesVisited = budget.visited
}

// tsLayerBypassSourceResult is one source's contribution to a
// tsLayerBypassSearchResult; tsLayerBypassRunSearch accumulates it across
// every source.
type tsLayerBypassSourceResult struct {
	witnesses            []LayerBypassWitness
	evaluated            int
	truncatedPairs       int
	truncatedSearch      bool
	unclassifiedNodeSeen bool
}

// tsLayerBypassSearchFromSource runs source's BFS shortest-path tree and
// evaluates every sink against it. budget is the shared node-visit counter
// across every source in the search.
func tsLayerBypassSearchFromSource(
	ctx context.Context,
	source string,
	sinks []string,
	adjacency map[string][]string,
	nodePositions map[string]layerBypassNodePosition,
	requiredLayer BypassLayer,
	budget *bfsBudget,
) tsLayerBypassSourceResult {
	var result tsLayerBypassSourceResult
	parents, hitBudget := budget.shortestPaths(ctx, source, adjacency)
	if hitBudget {
		result.truncatedSearch = true
	}
	skip := hitBudget || ctx.Err() != nil
	for _, sink := range sinks {
		if skip {
			result.truncatedPairs++
			continue
		}
		result.evaluated++
		stepPath, ok := reconstructReachabilityPath(parents, source, sink)
		if !ok {
			continue
		}
		if !stepPathFullyClassified(stepPath, nodePositions) {
			result.unclassifiedNodeSeen = true
			continue
		}
		result.witnesses = append(result.witnesses, LayerBypassWitness{
			ID:               fmt.Sprintf("bypass:%s:%s->%s@%s", requiredLayer.Name, source, sink, TSLayerBypassAlgorithm),
			Source:           source,
			Sink:             sink,
			RequiredLayer:    requiredLayer.Name,
			Path:             layerBypassSteps(stepPath, nodePositions),
			Confidence:       LayerBypassConfidenceHigh,
			AlgorithmVersion: TSLayerBypassAlgorithm,
		})
	}
	return result
}
