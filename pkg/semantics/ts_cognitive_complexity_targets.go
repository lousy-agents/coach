package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func scoredTSCCTarget(n engine.Node, source []byte) (tsCCTarget, bool) {
	if !isTSCCScoredKind(n.Kind()) {
		return tsCCTarget{}, false
	}
	// Overload signatures and abstract methods have no body — skip.
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

func isTSCCScoredKind(kind string) bool {
	switch kind {
	case "function_declaration", "generator_function_declaration",
		"function_expression", "generator_function",
		"arrow_function", "method_definition":
		return true
	default:
		return false
	}
}

func tsCCKind(nodeKind string) string {
	switch nodeKind {
	case "function_declaration", "generator_function_declaration":
		return "function"
	case "method_definition":
		return "method"
	case "function_expression", "generator_function":
		return "func_lit"
	case "arrow_function":
		return "arrow"
	default:
		return ""
	}
}

// tsIsNestedInScoredBody reports whether n sits lexically inside another
// scored function/method/arrow/func_lit (class bodies do not count).
func tsIsNestedInScoredBody(n engine.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if isTSCCScoredKind(p.Kind()) {
			return true
		}
	}
	return false
}

func tsCCBody(n engine.Node) engine.Node {
	if n == nil {
		return nil
	}
	return n.ChildByFieldName("body")
}
