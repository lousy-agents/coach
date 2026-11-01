package semantics

import (
	"unicode"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

var tsTransparentWrapperKinds = map[string]bool{
	"parenthesized_expression": true,
	"as_expression":            true,
	"satisfies_expression":     true,
	"non_null_expression":      true,
}

// tsAssignedName returns the name fn is held under: the variable it is
// bound to, the assignment target's property or element key, the class field
// it initializes, or the object property it is the value of, seen through
// parentheses, `as`, `satisfies`, and non-null assertions. It returns "" for
// a function held in none of those ways, such as a call argument or an
// immediately invoked function, and for a key tsKeyName cannot name.
func tsAssignedName(fn engine.Node, source []byte) string {
	held := tsHeldExpression(fn)
	if bound := tsBoundIdentifierName(held, source); bound != tsAnonymousFunctionOwnerName {
		return bound
	}
	parent := held.Parent()
	if parent == nil {
		return ""
	}
	var key, value engine.Node
	switch parent.Kind() {
	case "public_field_definition":
		key, value = parent.ChildByFieldName("name"), parent.ChildByFieldName("value")
	case "pair":
		key, value = parent.ChildByFieldName("key"), parent.ChildByFieldName("value")
	case "assignment_expression":
		key, value = tsMemberKeyNode(parent.ChildByFieldName("left")), parent.ChildByFieldName("right")
	default:
		return ""
	}
	if key == nil || value == nil || !sameNodeSpan(value, held) {
		return ""
	}
	return tsKeyName(key, source)
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

// tsMemberKeyNode returns the key of an assignment target: the property of
// `obj.key` or the index of `obj[key]`. It returns nil for any other target.
func tsMemberKeyNode(target engine.Node) engine.Node {
	if target == nil {
		return nil
	}
	switch target.Kind() {
	case "member_expression":
		return target.ChildByFieldName("property")
	case "subscript_expression":
		return target.ChildByFieldName("index")
	}
	return nil
}

// tsKeyName names a property key: an identifier or a number by its text, a
// string literal by its content, and the same inside computed brackets. A
// string whose content is a plain identifier or number loses its quotes, so
// `mark`, "mark", and ["mark"] are one holder; any other string keeps them
// ("A.run", "get value", "[KEY]"), so a key cannot spell a name that class
// qualification or brackets produce. A computed key that is a bare
// identifier keeps its brackets ([KEY]), because it names a variable, not a
// property. Any other computed key has no name and returns "".
func tsKeyName(key engine.Node, source []byte) string {
	switch key.Kind() {
	case "property_identifier", "private_property_identifier", "number":
		return key.Utf8Text(source)
	case "string":
		text := key.Utf8Text(source)
		if len(text) < 2 {
			return ""
		}
		content := text[1 : len(text)-1]
		if tsIsPlainKeyText(content) {
			return content
		}
		return `"` + content + `"`
	case "identifier":
		return "[" + key.Utf8Text(source) + "]"
	case "computed_property_name":
		if inner := key.Child(1); inner != nil {
			return tsKeyName(inner, source)
		}
	}
	return ""
}

// tsIsPlainKeyText reports whether s is written the same as an unquoted
// property key: a run of decimal digits, or an identifier.
func tsIsPlainKeyText(s string) bool {
	if s == "" {
		return false
	}
	digits := true
	for _, r := range s {
		if r < '0' || r > '9' {
			digits = false
			break
		}
	}
	if digits {
		return true
	}
	for i, r := range s {
		isStart := unicode.IsLetter(r) || r == '_' || r == '$'
		if !isStart && (i == 0 || !unicode.IsDigit(r)) {
			return false
		}
	}
	return true
}
