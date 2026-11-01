package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

var tsTransparentWrapperKinds = map[string]bool{
	"parenthesized_expression": true,
	"as_expression":            true,
	"satisfies_expression":     true,
	"non_null_expression":      true,
}

// tsAssignedName returns the name fn is held under: the variable it is
// bound to, the assignment target's property name, the class field it
// initializes, or the object property it is the value of, seen through
// parentheses, `as`, `satisfies`, and non-null assertions. It returns "" for
// a function held in none of those ways, such as a call argument or an
// immediately invoked function.
func tsAssignedName(fn engine.Node, source []byte) string {
	held := tsHeldExpression(fn)
	if bound := tsBoundIdentifierName(held, source); bound != tsAnonymousFunctionOwnerName {
		return bound
	}
	parent := held.Parent()
	if parent == nil {
		return ""
	}
	var name, value engine.Node
	switch parent.Kind() {
	case "public_field_definition":
		name, value = parent.ChildByFieldName("name"), parent.ChildByFieldName("value")
	case "pair":
		name, value = parent.ChildByFieldName("key"), parent.ChildByFieldName("value")
	case "assignment_expression":
		name, value = tsMemberPropertyName(parent.ChildByFieldName("left")), parent.ChildByFieldName("right")
	default:
		return ""
	}
	if name == nil || value == nil || !sameNodeSpan(value, held) {
		return ""
	}
	switch name.Kind() {
	case "property_identifier", "private_property_identifier":
		return name.Utf8Text(source)
	}
	return ""
}

// tsHeldExpression returns the outermost parenthesis, `as`, `satisfies`, or
// non-null wrapper around fn, or fn itself when it has none: the node that
// is actually bound, assigned, or initialized.
func tsHeldExpression(fn engine.Node) engine.Node {
	held := fn
	for parent := held.Parent(); parent != nil && tsTransparentWrapperKinds[parent.Kind()]; parent = held.Parent() {
		held = parent
	}
	return held
}

func tsMemberPropertyName(target engine.Node) engine.Node {
	if target == nil || target.Kind() != "member_expression" {
		return nil
	}
	return target.ChildByFieldName("property")
}
