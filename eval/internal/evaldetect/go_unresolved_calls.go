package evaldetect

// dropCallsOutsideTheGraph is a guard: every declared function has a node
// and an interface call is unresolved before it gets here, so no call should
// reach a missing node. If one does, it is counted as unresolved rather than
// followed into a nil node.
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
