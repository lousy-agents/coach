package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func tsCCName(n engine.Node, source []byte) string {
	switch n.Kind() {
	case "function_declaration", "generator_function_declaration", "method_definition":
		if name := n.ChildByFieldName("name"); name != nil {
			return name.Utf8Text(source)
		}
		return "<func lit>"
	case "function_expression", "generator_function":
		// Prefer the expression's own name identifier when present.
		if name := n.ChildByFieldName("name"); name != nil {
			return name.Utf8Text(source)
		}
		return tsBoundIdentifierName(n, source)
	case "arrow_function":
		return tsBoundIdentifierName(n, source)
	default:
		return "<func lit>"
	}
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
