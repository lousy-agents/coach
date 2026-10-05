package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// reactWalkLocalScope visits n and its descendants in pre-order but does
// not descend past a nested function/arrow boundary -- unlike
// reactWalkScope, it has no PascalCase-component exception, since any
// function boundary already stops it. Used to attribute calls to the
// function body that directly contains them, not an enclosing one.
func reactWalkLocalScope(n engine.Node, visit func(engine.Node)) {
	if n == nil {
		return
	}
	visit(n)
	switch n.Kind() {
	case "function_declaration", "function_expression", "arrow_function":
		return
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		reactWalkLocalScope(n.Child(i), visit)
	}
}
func reactTransitionJSXHandlerName(parent engine.Node, source []byte) (string, bool) {
	if parent.Kind() != "jsx_expression" {
		return "", false
	}
	attr := parent.Parent()
	if attr == nil || attr.Kind() != "jsx_attribute" {
		return "", false
	}
	n := reactJSXAttributeNameText(attr, source)
	if !reactJSXOnAttrPattern.MatchString(n) {
		return "", false
	}
	return n, true
}
func reactTransitionEffect(fn, parent engine.Node, source []byte) (kind, name string, ok bool) {
	if parent == nil || parent.Kind() != "arguments" || !sameNodeSpan(reactFirstArgumentNode(parent), fn) {
		return "", "", false
	}
	call := parent.Parent()
	if call == nil || call.Kind() != "call_expression" || !reactIsEffectHookCallee(call, source) {
		return "", "", false
	}
	return "effect", reactTransitionNameOrAnonymous(fn, source), true
}
func reactUseStateSetterIndex(useState []ReactUseStateBinding) map[string]string {
	setters := map[string]string{}
	for _, u := range useState {
		if u.Setter != "" && u.Binding != "" {
			setters[u.Setter] = u.Binding
		}
	}
	return setters
}
