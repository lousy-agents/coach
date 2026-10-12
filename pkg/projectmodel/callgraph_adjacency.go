package projectmodel

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
		kept := keptNeighbors(tos, removed)
		if len(kept) > 0 {
			out[from] = kept
		}
	}
	return out
}

func keptNeighbors(tos []string, removed map[string]bool) []string {
	var kept []string
	for _, to := range tos {
		if removed[to] {
			continue
		}
		kept = append(kept, to)
	}
	return kept
}
