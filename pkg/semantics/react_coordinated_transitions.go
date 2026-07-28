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

func reactUseStateSetterIndex(useState []ReactUseStateBinding) map[string]string {
	setters := map[string]string{}
	for _, u := range useState {
		if u.Setter != "" && u.Binding != "" {
			setters[u.Setter] = u.Binding
		}
	}
	return setters
}

func reactLocalFunctions(body engine.Node, source []byte) []engine.Node {
	var funcs []engine.Node
	reactWalkScope(body, source, func(n engine.Node) {
		switch n.Kind() {
		case "function_declaration", "function_expression", "arrow_function":
			if reactIsNestedComponent(n, source) {
				return
			}
			funcs = append(funcs, n)
		}
	})
	return funcs
}

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

// reactTransitionNameOrAnonymous returns fn's own syntactic name field when
// present, otherwise the epic's "<anonymous>" sentinel.
func reactTransitionNameOrAnonymous(fn engine.Node, source []byte) string {
	if own := reactFuncOwnName(fn, source); own != "" {
		return own
	}
	return "<anonymous>"
}
