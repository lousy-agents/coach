package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// reactUseStateBindingNames resolves call's binding/setter pair from its
// enclosing variable_declarator, when call is exactly that declarator's own
// initializer: `const [x, setX] = useState(...)` -> ("x", "setX") when the
// destructure's 2nd element is a plain identifier, else ("x", "");
// `const x = useState(...)` -> ("x", ""); anything else (call not assigned
// via a declarator, or an unsupported name pattern) -> ("", "").
func reactUseStateBindingNames(call engine.Node, source []byte) (string, string) {
	parent := call.Parent()
	if parent == nil || parent.Kind() != "variable_declarator" {
		return "", ""
	}
	value := parent.ChildByFieldName("value")
	if value == nil || !sameNodeSpan(value, call) {
		return "", ""
	}
	name := parent.ChildByFieldName("name")
	if name == nil {
		return "", ""
	}
	switch name.Kind() {
	case "identifier":
		return name.Utf8Text(source), ""
	case "array_pattern":
		elems := reactArrayPatternElements(name)
		binding, setter := "", ""
		if len(elems) > 0 && elems[0] != nil && elems[0].Kind() == "identifier" {
			binding = elems[0].Utf8Text(source)
		}
		if len(elems) > 1 && elems[1] != nil && elems[1].Kind() == "identifier" {
			setter = elems[1].Utf8Text(source)
		}
		return binding, setter
	default:
		return "", ""
	}
}

// moduleHasUseClientDirective reports whether root's first non-comment
// top-level statement is an expression_statement whose sole expression is
// the string literal "use client" (either quote style).
func moduleHasUseClientDirective(root engine.Node, source []byte) bool {
	count := root.ChildCount()
	for i := 0; i < count; i++ {
		c := root.Child(i)
		if c.Kind() == "comment" {
			continue
		}
		if c.Kind() != "expression_statement" {
			return false
		}
		str := reactFirstStringChild(c)
		if str == nil {
			return false
		}
		return reactIsUseClientLiteral(str, source)
	}
	return false
}
