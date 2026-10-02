package semantics

import (
	"sort"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// reactExtractUseState collects every direct useState()/React.useState()
// call within body's scope (per reactWalkScope's nesting rules), ordered by
// call start_byte ascending.
func reactExtractUseState(body engine.Node, source []byte) []ReactUseStateBinding {
	var calls []engine.Node
	reactWalkScope(body, source, func(n engine.Node) {
		if n.Kind() != "call_expression" {
			return
		}
		if isReactUseStateCallee(n, source) {
			calls = append(calls, n)
		}
	})
	sort.SliceStable(calls, func(i, j int) bool {
		return calls[i].StartByte() < calls[j].StartByte()
	})

	out := make([]ReactUseStateBinding, 0, len(calls))
	for _, call := range calls {
		binding, setter := reactUseStateBindingNames(call, source)
		out = append(out, ReactUseStateBinding{
			Binding:  binding,
			Setter:   setter,
			Location: locationFromNode(call),
		})
	}
	return out
}

// isReactUseStateCallee reports whether call's callee is exactly useState
// or React.useState. Aliased imports (`import { useState as us }`) are a
// deliberately accepted false negative -- this package does not resolve
// import aliases.
func isReactUseStateCallee(call engine.Node, source []byte) bool {
	fn := call.ChildByFieldName("function")
	if fn == nil {
		return false
	}
	switch fn.Kind() {
	case "identifier":
		return fn.Utf8Text(source) == "useState"
	case "member_expression":
		obj := fn.ChildByFieldName("object")
		prop := fn.ChildByFieldName("property")
		return obj != nil && prop != nil && obj.Kind() == "identifier" && obj.Utf8Text(source) == "React" && prop.Utf8Text(source) == "useState"
	default:
		return false
	}
}

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

// reactArrayPatternElements returns n's elements by position, including a
// nil placeholder for an elided element (e.g. `[, setC]`), so that index 0
// is always the binding slot and index 1 the setter slot even when earlier
// elements are holes.
func reactArrayPatternElements(n engine.Node) []engine.Node {
	var out []engine.Node
	count := n.ChildCount()
	pendingElement := false
	for i := 0; i < count; i++ {
		c := n.Child(i)
		switch c.Kind() {
		case "[", "]":
			continue
		case ",":
			out = reactArrayPatternOnComma(out, pendingElement)
			pendingElement = false
		default:
			out = append(out, c)
			pendingElement = true
		}
	}
	return out
}

func reactArrayPatternOnComma(out []engine.Node, pendingElement bool) []engine.Node {
	if pendingElement {
		return out
	}
	return append(out, nil)
}
