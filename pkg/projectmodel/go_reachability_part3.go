package projectmodel

import (
	"context"
)

// shortestPaths runs a single breadth-first traversal from source over
// adjacency (whose neighbor lists must already be sorted), returning a
// parent map spanning every node reached before a budget or context
// deadline stopped the walk. Because neighbors are visited in sorted order
// and each node is enqueued at most once (on first discovery), the
// resulting shortest-path tree is deterministic even when multiple
// equal-length paths exist.
func (b *bfsBudget) shortestPaths(ctx context.Context, source string, adjacency map[string][]string) (map[string]string, bool) {
	parents := map[string]string{source: ""}
	visited := map[string]bool{source: true}
	queue := []string{source}

	for len(queue) > 0 {
		if ctx.Err() != nil {
			return parents, true
		}
		if b.max > 0 && b.visited >= b.max {
			return parents, true
		}
		node := queue[0]
		queue = queue[1:]
		b.visited++

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
