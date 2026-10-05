package semantics

import (
	"regexp"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// reactHandlerNamePattern matches an identifier/property/attribute name
// shaped like an event handler binding (onX, handleX), case-insensitively
// on the prefix.
var reactHandlerNamePattern = regexp.MustCompile(`(?i)^(on|handle)`)

// reactJSXOnAttrPattern matches a JSX attribute name shaped like an event
// handler prop (onClick, onSelect, ...).
var reactJSXOnAttrPattern = regexp.MustCompile(`^on[A-Z]`)

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
