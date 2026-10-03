package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"

	"sort"
)

// reactTransitionKindAndName classifies fn as "effect" (first argument of
// useEffect/useLayoutEffect), "handler" (bound to an on*/handle* name via a
// declarator, assignment, object property, or JSX on* attribute), or
// "callback" (none of the above). Name is the assigned identifier, JSX
// attribute name, function's own name field, or the "<anonymous>" sentinel
// when none of those exist — never the empty string.
func reactTransitionKindAndName(fn engine.Node, source []byte) (kind, name string) {
	parent := fn.Parent()
	if kind, name, ok := reactTransitionEffect(fn, parent, source); ok {
		return kind, name
	}
	if parent == nil {
		return "callback", reactTransitionNameOrAnonymous(fn, source)
	}
	if kind, name, ok := reactTransitionBoundKind(parent, source); ok {
		return kind, name
	}
	if name, ok := reactTransitionJSXHandlerName(parent, source); ok {
		return "handler", name
	}
	return "callback", reactTransitionNameOrAnonymous(fn, source)
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

// reactExtractCoordinatedTransitions finds every function/arrow body in
// body's scan set (reactWalkScope) that calls at least two distinct
// useState setters, and returns one ReactCoordinatedTransition per such
// body, ordered by the body's own start_byte. Calls to the same setter
// within one body count once; setters not present in useState (never
// resolved, e.g. destructure fell through to "") never match.
func reactExtractCoordinatedTransitions(body engine.Node, source []byte, useState []ReactUseStateBinding) []ReactCoordinatedTransition {
	setters := reactUseStateSetterIndex(useState)
	if len(setters) == 0 {
		return nil
	}

	var out []ReactCoordinatedTransition
	for _, fn := range reactLocalFunctions(body, source) {
		updated := reactBindingsUpdatedBy(fn, source, setters)
		if len(updated) < 2 {
			continue
		}
		kind, name := reactTransitionKindAndName(fn, source)
		out = append(out, ReactCoordinatedTransition{
			Name:            name,
			Kind:            kind,
			Location:        locationFromNode(fn),
			UpdatedBindings: updated,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Location.StartByte < out[j].Location.StartByte
	})
	return out
}
