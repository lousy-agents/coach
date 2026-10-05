package semantics

import "github.com/lousy-agents/coach/pkg/semantics/internal/engine"

func anyNewExpressionHasConstructorField(n engine.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind() == "new_expression" && n.ChildByFieldName("constructor") != nil {
		return true
	}
	for i := 0; i < n.ChildCount(); i++ {
		if anyNewExpressionHasConstructorField(n.Child(i)) {
			return true
		}
	}
	return false
}
