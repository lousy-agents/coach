package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// mutatingTSMethodNames is the exact set of built-in Array/Map/Set method
// names whose call on a parameter-rooted receiver is treated as an
// in-place mutation of that parameter (Story 2). Arbitrary custom methods
// (e.g. `user.setName()`) are deliberately not in this set and so are
// never flagged.
var mutatingTSMethodNames = map[string]bool{
	"copyWithin": true,
	"fill":       true,
	"pop":        true,
	"push":       true,
	"reverse":    true,
	"shift":      true,
	"sort":       true,
	"splice":     true,
	"unshift":    true,
	"set":        true,
	"add":        true,
	"delete":     true,
	"clear":      true,
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
