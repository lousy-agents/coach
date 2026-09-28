package projectmodel

import (
	"context"
)

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

// removeLayerNodesFromAdjacency returns adjacency with every node in
// removed deleted, both as an edge source and as an edge destination, so a
// BFS over the result can only find a path that never touches one of those
// nodes. Surviving neighbor lists stay in the same sorted order adjacency
// already used, preserving bfsShortestPaths' deterministic tie-breaking.
func removeLayerNodesFromAdjacency(adjacency map[string][]string, removed map[string]bool) map[string][]string {
	out := make(map[string][]string, len(adjacency))
	for from, tos := range adjacency {
		if removed[from] {
			continue
		}
		var kept []string
		for _, to := range tos {
			if removed[to] {
				continue
			}
			kept = append(kept, to)
		}
		if len(kept) > 0 {
			out[from] = kept
		}
	}
	return out
}
