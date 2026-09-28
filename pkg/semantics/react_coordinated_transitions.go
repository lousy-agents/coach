package semantics

import (
	"regexp"
	"sort"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// reactHandlerNamePattern matches an identifier/property/attribute name
// shaped like an event handler binding (onX, handleX), case-insensitively
// on the prefix.
var reactHandlerNamePattern = regexp.MustCompile(`(?i)^(on|handle)`)

// reactJSXOnAttrPattern matches a JSX attribute name shaped like an event
// handler prop (onClick, onSelect, ...).
var reactJSXOnAttrPattern = regexp.MustCompile(`^on[A-Z]`)

// reactExtractCoordinatedTransitions finds every function/arrow body in
// body's scan set (reactWalkScope) that calls at least two distinct
// useState setters, and returns one ReactCoordinatedTransition per such
// body, ordered by the body's own start_byte. Calls to the same setter
// within one body count once; setters not present in useState (never
// resolved, e.g. destructure fell through to "") never match.

func reactBindingsUpdatedBy(fn engine.Node, source []byte, setters map[string]string) []string {
	updated := map[string]struct{}{}
	reactWalkLocalScope(fn.ChildByFieldName("body"), func(n engine.Node) {
		if n.Kind() != "call_expression" {
			return
		}
		callee := n.ChildByFieldName("function")
		if callee == nil || callee.Kind() != "identifier" {
			return
		}
		if binding, ok := setters[callee.Utf8Text(source)]; ok {
			updated[binding] = struct{}{}
		}
	})
	if len(updated) == 0 {
		return nil
	}
	bindings := make([]string, 0, len(updated))
	for b := range updated {
		bindings = append(bindings, b)
	}
	sort.Strings(bindings)
	return bindings
}

// reactWalkLocalScope visits n and its descendants in pre-order but does
// not descend past a nested function/arrow boundary -- unlike
// reactWalkScope, it has no PascalCase-component exception, since any
// function boundary already stops it. Used to attribute calls to the
// function body that directly contains them, not an enclosing one.

// reactIsEffectHookCallee reports whether call's callee is exactly one of
// the four supported effect hooks: useEffect, useLayoutEffect,
// React.useEffect, or React.useLayoutEffect.

// reactTransitionKindAndName classifies fn as "effect" (first argument of
// useEffect/useLayoutEffect), "handler" (bound to an on*/handle* name via a
// declarator, assignment, object property, or JSX on* attribute), or
// "callback" (none of the above). Name is the assigned identifier, JSX
// attribute name, function's own name field, or the "<anonymous>" sentinel
// when none of those exist — never the empty string.

func reactTransitionBoundKind(parent engine.Node, source []byte) (kind, name string, ok bool) {
	var n string
	switch parent.Kind() {
	case "variable_declarator":
		nameNode := parent.ChildByFieldName("name")
		if nameNode == nil || nameNode.Kind() != "identifier" {
			return "", "", false
		}
		n = nameNode.Utf8Text(source)
	case "assignment_expression":
		left := parent.ChildByFieldName("left")
		if left == nil || left.Kind() != "identifier" {
			return "", "", false
		}
		n = left.Utf8Text(source)
	case "pair":
		key := parent.ChildByFieldName("key")
		if key == nil || key.Kind() != "property_identifier" {
			return "", "", false
		}
		n = key.Utf8Text(source)
	default:
		return "", "", false
	}
	if reactHandlerNamePattern.MatchString(n) {
		return "handler", n, true
	}
	return "callback", n, true
}

// reactTransitionNameOrAnonymous returns fn's own syntactic name field when
// present, otherwise the epic's "<anonymous>" sentinel.
