package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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

// reactIsEffectHookCallee reports whether call's callee is exactly one of
// the four supported effect hooks: useEffect, useLayoutEffect,
// React.useEffect, or React.useLayoutEffect.
func reactIsEffectHookCallee(call engine.Node, source []byte) bool {
	fn := call.ChildByFieldName("function")
	if fn == nil {
		return false
	}
	switch fn.Kind() {
	case "identifier":
		name := fn.Utf8Text(source)
		return name == "useEffect" || name == "useLayoutEffect"
	case "member_expression":
		obj := fn.ChildByFieldName("object")
		prop := fn.ChildByFieldName("property")
		if obj == nil || prop == nil || obj.Kind() != "identifier" || obj.Utf8Text(source) != "React" {
			return false
		}
		name := prop.Utf8Text(source)
		return name == "useEffect" || name == "useLayoutEffect"
	default:
		return false
	}
}
