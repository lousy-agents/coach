package projectmodel

import (
	"context"
)

// bfsBudget is the shared node-visit counter for shortest-path walks.
// visited is not reset per source: max bounds the total number of nodes
// dequeued across the whole search.
type bfsBudget struct {
	visited int
	max     int
}

// shortestPaths runs a single breadth-first traversal from source over
// adjacency (whose neighbor lists must already be sorted), returning a
// parent map spanning every node reached before a budget or context
// deadline stopped the walk. Because neighbors are visited in sorted order
// and each node is enqueued at most once (on first discovery), the
// resulting shortest-path tree is deterministic even when multiple
// equal-length paths exist.
func (b *bfsBudget) shortestPaths(ctx context.Context, source string, adjacency map[string][]string) (map[string]string, bool) {
	frontier := &bfsFrontier{
		parents: map[string]string{source: ""},
		visited: map[string]bool{source: true},
		queue:   []string{source},
	}

	for len(frontier.queue) > 0 {
		if ctx.Err() != nil {
			return frontier.parents, true
		}
		if b.max > 0 && b.visited >= b.max {
			return frontier.parents, true
		}
		node := frontier.queue[0]
		frontier.queue = frontier.queue[1:]
		b.visited++
		frontier.enqueue(adjacency[node], node)
	}
	return frontier.parents, false
}

type bfsFrontier struct {
	parents map[string]string
	visited map[string]bool
	queue   []string
}

func (f *bfsFrontier) enqueue(nexts []string, node string) {
	for _, next := range nexts {
		if f.visited[next] {
			continue
		}
		f.visited[next] = true
		f.parents[next] = node
		f.queue = append(f.queue, next)
	}
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
