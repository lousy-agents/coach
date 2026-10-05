package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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
func flattenTSBooleanOps(n engine.Node) []string {
	n = unwrapTSParen(n)
	if !isTSBooleanBinary(n) {
		return nil
	}
	op := tsBinaryOp(n)
	left := flattenTSBooleanOps(n.ChildByFieldName("left"))
	right := flattenTSBooleanOps(n.ChildByFieldName("right"))
	out := make([]string, 0, len(left)+1+len(right))
	out = append(out, left...)
	out = append(out, op)
	out = append(out, right...)
	return out
}
