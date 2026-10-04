package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func expressionListIsSingleNode(list engine.Node, want engine.Node) bool {
	if list == nil || want == nil {
		return false
	}
	var found engine.Node
	count := list.ChildCount()
	for i := 0; i < count; i++ {
		child := list.Child(i)
		if child.Kind() == "," {
			continue
		}
		if found != nil {
			return false
		}
		found = child
	}
	return found != nil && sameNodeSpan(found, want)
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
func goHasLabel(n engine.Node) bool {
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		if n.Child(i).Kind() == "label_name" {
			return true
		}
	}
	return false
}
