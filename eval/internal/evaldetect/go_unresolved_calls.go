package evaldetect

// dropCallsOutsideTheGraph counts a call to a function that has no node,
// such as an interface method, as unresolved, so the search only follows
// calls it can continue from.
func (graph *callGraph) dropCallsOutsideTheGraph() {
	for _, node := range graph.nodes {
		var dropped int
		node.calls, dropped = graph.callsInTheGraph(node.calls)
		graph.unresolved += dropped
	}
}

func (graph *callGraph) callsInTheGraph(calls []callSite) ([]callSite, int) {
	kept := calls[:0]
	for _, call := range calls {
		if graph.nodes[call.callee] != nil {
			kept = append(kept, call)
		}
	}
	return kept, len(calls) - len(kept)
}

// unfollowedBodylessCalls counts the calls to a function declared without a
// body outside the downstream layers: the search cannot see what such a
// function calls. A bodyless downstream function is a sink, so a call to it
// is fully resolved.
func unfollowedBodylessCalls(graph callGraph, policy Policy) int {
	_, downstream := policy.bypassLayers()
	unfollowed := func(callee *funcNode) bool {
		layer, _ := policy.layerOf(callee.dir)
		return callee.bodyless && !downstream[layer]
	}
	count := 0
	for _, node := range graph.nodes {
		count += graph.countCalls(node, unfollowed)
	}
	return count
}

func (graph callGraph) countCalls(node *funcNode, match func(*funcNode) bool) int {
	count := 0
	for _, call := range node.calls {
		if match(graph.nodes[call.callee]) {
			count++
		}
	}
	return count
}
