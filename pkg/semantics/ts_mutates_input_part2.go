package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func (c *tsFeatureCollector) recordMutatesInputAssignmentTargets(n engine.Node, source []byte, scopes []tsParamScope) {
	if n == nil {
		return
	}
	if base := tsMutationBase(n); base != nil {
		c.recordMutatesInput(base, n, source, scopes)
		return
	}
	switch n.Kind() {
	case "parenthesized_expression":
		c.recordMutatesInputAssignmentTargets(tsWrappedExpressionInner(n), source, scopes)
	case "pair_pattern":
		c.recordMutatesInputAssignmentTargets(n.ChildByFieldName("value"), source, scopes)
	case "assignment_pattern":
		c.recordMutatesInputAssignmentTargets(n.ChildByFieldName("left"), source, scopes)
	case "rest_pattern":
		c.recordMutatesInputAssignmentTargets(n.ChildByFieldName("argument"), source, scopes)
	case "object_pattern", "array_pattern":
		count := n.ChildCount()
		for i := 0; i < count; i++ {
			c.recordMutatesInputAssignmentTargets(n.Child(i), source, scopes)
		}
	}
}

// checkMutatesInputCall emits a "mutates_input" Finding (Story 2) if n (a
// call_expression) calls one of mutatingTSMethodNames on a receiver
// rooted at some enclosing scope's identifier-bound parameter, either
// directly (`p.push(x)`, `arr.sort()`, `m.set(k, v)`) or through a chain of
// nested member/subscript accesses (`p.items.push(1)`). Arbitrary custom
// method calls (`user.setName()`) are not in mutatingTSMethodNames and so
// never match. Evidence/Location are taken from fn (the receiver.method
// member_expression, e.g. "p.items.push"), not the whole call_expression:
// a call's arguments can be arbitrarily long or complex
// (`p.items.push(someVeryLargeExpression())`), which would conflict with
// Evidence staying short, and would also diverge from the Go detector's
// bounded, target-only evidence.
func (c *tsFeatureCollector) checkMutatesInputCall(n engine.Node, source []byte, scopes []tsParamScope) {
	fn := n.ChildByFieldName("function")
	if fn == nil {
		return
	}
	object, methodName := tsMutatingMethodReceiver(fn, source)
	if object == nil || !mutatingTSMethodNames[methodName] {
		return
	}
	base := tsResolveRootIdentifier(object)
	if base == nil {
		return
	}
	c.recordMutatesInput(base, fn, source, scopes)
}
func tsMutatingMethodReceiver(fn engine.Node, source []byte) (engine.Node, string) {
	switch fn.Kind() {
	case "member_expression":
		property := fn.ChildByFieldName("property")
		if property == nil {
			return nil, ""
		}
		return fn.ChildByFieldName("object"), property.Utf8Text(source)
	case "subscript_expression":
		index := fn.ChildByFieldName("index")
		methodName, ok := tsStringLiteralText(index, source)
		if !ok {
			return nil, ""
		}
		return fn.ChildByFieldName("object"), methodName
	default:
		return nil, ""
	}
}
