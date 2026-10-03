package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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

func scoredTSCCTarget(n engine.Node, source []byte) (tsCCTarget, bool) {
	if !isTSCCScoredKind(n.Kind()) {
		return tsCCTarget{}, false
	}
	if tsCCBody(n) == nil {
		return tsCCTarget{}, false
	}
	return tsCCTarget{
		node:     n,
		name:     tsCCName(n, source),
		kind:     tsCCKind(n.Kind()),
		topLevel: !tsIsNestedInScoredBody(n),
	}, true
}

// tsBoundIdentifierName returns the single identifier a lit/arrow is bound to
// via variable_declarator or assignment_expression; otherwise "<func lit>".
func tsBoundIdentifierName(lit engine.Node, source []byte) string {
	parent := lit.Parent()
	if parent == nil {
		return "<func lit>"
	}
	switch parent.Kind() {
	case "variable_declarator":
		name := parent.ChildByFieldName("name")
		value := parent.ChildByFieldName("value")
		if name == nil || name.Kind() != "identifier" || value == nil || !sameNodeSpan(value, lit) {
			return "<func lit>"
		}
		return name.Utf8Text(source)
	case "assignment_expression":
		left := parent.ChildByFieldName("left")
		right := parent.ChildByFieldName("right")
		if left == nil || left.Kind() != "identifier" || right == nil || !sameNodeSpan(right, lit) {
			return "<func lit>"
		}
		return left.Utf8Text(source)
	default:
		return "<func lit>"
	}
}
func tsCCBody(n engine.Node) engine.Node {
	if n == nil {
		return nil
	}
	return n.ChildByFieldName("body")
}
