package semantics

import (
	"sort"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// computeGoCognitiveComplexity discovers every scored Go function body under
// root and returns one FunctionCognitiveComplexity record per body (including
// zero scores), ordered by ascending location.start_byte then name.
func computeGoCognitiveComplexity(root engine.Node, source []byte) []FunctionCognitiveComplexity {
	if root == nil {
		return nil
	}
	targets := collectGoCCTargets(root, source)
	if len(targets) == 0 {
		return nil
	}
	records := make([]FunctionCognitiveComplexity, 0, len(targets))
	for _, t := range targets {
		body := t.node.ChildByFieldName("body")
		score := 0
		if body != nil {
			s := &goCCScorer{source: source, funcName: t.name}
			s.walk(body, 0)
			score = s.score
		}
		records = append(records, FunctionCognitiveComplexity{
			Name:     t.name,
			Kind:     t.kind,
			Location: locationFromNode(t.node),
			Score:    score,
		})
	}
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].Location.StartByte != records[j].Location.StartByte {
			return records[i].Location.StartByte < records[j].Location.StartByte
		}
		return records[i].Name < records[j].Name
	})
	return records
}

type goCCTarget struct {
	node engine.Node
	name string
	kind string
}

func collectGoCCTargets(root engine.Node, source []byte) []goCCTarget {
	if root == nil {
		return nil
	}
	// Iterative DFS into one result slice: avoids recursive slice-concat
	// copies on large ASTs. Discovery order is irrelevant — callers sort by
	// location.start_byte then name.
	var out []goCCTarget
	stack := []engine.Node{root}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch n.Kind() {
		case "function_declaration":
			out = append(out, goCCTarget{node: n, name: goDeclName(n, source), kind: "function"})
		case "method_declaration":
			out = append(out, goCCTarget{node: n, name: goDeclName(n, source), kind: "method"})
		case "func_literal":
			out = append(out, goCCTarget{node: n, name: goFuncLitName(n, source), kind: "func_lit"})
		}
		count := n.ChildCount()
		for i := count - 1; i >= 0; i-- {
			stack = append(stack, n.Child(i))
		}
	}
	return out
}
