package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func reactLocalFunctionBindingName(n engine.Node, source []byte) (string, bool) {
	switch n.Kind() {
	case "function_declaration":
		if reactIsNestedComponent(n, source) {
			return "", false
		}
		name := n.ChildByFieldName("name")
		if name == nil {
			return "", false
		}
		nme := name.Utf8Text(source)
		if nme == "" {
			return "", false
		}
		return nme, true
	case "function_expression", "arrow_function":
		if reactIsNestedComponent(n, source) {
			return "", false
		}
		return reactBoundIdentifierName(n.Parent(), source)
	default:
		return "", false
	}
}

func reactBoundIdentifierName(parent engine.Node, source []byte) (string, bool) {
	if parent == nil {
		return "", false
	}
	switch parent.Kind() {
	case "variable_declarator":
		nameNode := parent.ChildByFieldName("name")
		if nameNode == nil || nameNode.Kind() != "identifier" {
			return "", false
		}
		nme := nameNode.Utf8Text(source)
		if nme == "" {
			return "", false
		}
		return nme, true
	case "assignment_expression":
		left := parent.ChildByFieldName("left")
		if left == nil || left.Kind() != "identifier" {
			return "", false
		}
		nme := left.Utf8Text(source)
		if nme == "" {
			return "", false
		}
		return nme, true
	default:
		return "", false
	}
}
