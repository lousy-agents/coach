package semantics

import (
	"sort"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// computeTSCognitiveComplexity discovers every scored TS/TSX function body
// under root and returns one record per body plus a parallel topLevel slice
// (true when the body is not lexically nested inside another scored body).
func computeTSCognitiveComplexity(root engine.Node, source []byte) ([]FunctionCognitiveComplexity, []bool) {
	if root == nil {
		return nil, nil
	}
	targets := collectTSCCTargets(root, source)
	if len(targets) == 0 {
		return nil, nil
	}
	records := make([]FunctionCognitiveComplexity, 0, len(targets))
	topLevel := make([]bool, 0, len(targets))
	for _, t := range targets {
		body := tsCCBody(t.node)
		score := 0
		if body != nil {
			s := &tsCCScorer{source: source, funcName: t.name}
			s.walk(body, 0, false)
			score = s.score
		}
		records = append(records, FunctionCognitiveComplexity{
			Name:     t.name,
			Kind:     t.kind,
			Location: locationFromNode(t.node),
			Score:    score,
		})
		topLevel = append(topLevel, t.topLevel)
	}
	// Stable order: start_byte then name (matches Go path / JSON contract).
	type pair struct {
		r FunctionCognitiveComplexity
		t bool
	}
	pairs := make([]pair, len(records))
	for i := range records {
		pairs[i] = pair{records[i], topLevel[i]}
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i].r.Location.StartByte != pairs[j].r.Location.StartByte {
			return pairs[i].r.Location.StartByte < pairs[j].r.Location.StartByte
		}
		return pairs[i].r.Name < pairs[j].r.Name
	})
	for i := range pairs {
		records[i] = pairs[i].r
		topLevel[i] = pairs[i].t
	}
	return records, topLevel
}

type tsCCTarget struct {
	node     engine.Node
	name     string
	kind     string
	topLevel bool
}

func collectTSCCTargets(root engine.Node, source []byte) []tsCCTarget {
	if root == nil {
		return nil
	}
	// Iterative DFS into one result slice: avoids recursive slice-concat
	// copies on large ASTs. Discovery order is irrelevant — callers sort by
	// location.start_byte then name.
	var out []tsCCTarget
	stack := []engine.Node{root}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if target, ok := scoredTSCCTarget(n, source); ok {
			out = append(out, target)
		}
		count := n.ChildCount()
		for i := count - 1; i >= 0; i-- {
			stack = append(stack, n.Child(i))
		}
	}
	return out
}
