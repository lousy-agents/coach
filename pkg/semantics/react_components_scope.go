package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// reactWalkScope visits n and its descendants in pre-order, but does not
// descend into a nested non-exported PascalCase-named function/arrow's
// subtree: that inner component is excluded entirely from the outer
// candidate's fact walk. A nested non-PascalCase helper's subtree remains
// fully visited -- state/calls inside it attribute to the outer candidate.
func reactWalkScope(n engine.Node, source []byte, visit func(engine.Node)) {
	if n == nil {
		return
	}
	visit(n)
	if reactIsNestedComponent(n, source) {
		return
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		reactWalkScope(n.Child(i), source, visit)
	}
}

func reactIsNestedComponent(n engine.Node, source []byte) bool {
	switch n.Kind() {
	case "function_declaration":
		if name := n.ChildByFieldName("name"); name != nil {
			return isPascalCaseName(name.Utf8Text(source))
		}
		return false
	case "function_expression":
		if name := n.ChildByFieldName("name"); name != nil {
			return isPascalCaseName(name.Utf8Text(source))
		}
		bound := tsBoundIdentifierName(n, source)
		return bound != "<func lit>" && isPascalCaseName(bound)
	case "arrow_function":
		bound := tsBoundIdentifierName(n, source)
		return bound != "<func lit>" && isPascalCaseName(bound)
	default:
		return false
	}
}

func reactScopeContainsJSX(body engine.Node, source []byte) bool {
	found := false
	reactWalkScope(body, source, func(n engine.Node) {
		if found {
			return
		}
		switch n.Kind() {
		case "jsx_element", "jsx_self_closing_element", "jsx_fragment":
			found = true
		}
	})
	return found
}

func reactScopeInvokesHook(body engine.Node, source []byte) bool {
	found := false
	reactWalkScope(body, source, func(n engine.Node) {
		if found || n.Kind() != "call_expression" {
			return
		}
		if reactIsHookCallee(n, source) {
			found = true
		}
	})
	return found
}
