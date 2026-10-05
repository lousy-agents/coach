package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"

	"sort"
)

func reactIsNestedComponent(n engine.Node, source []byte) bool {
	switch n.Kind() {
	case "function_declaration":
		if name := n.ChildByFieldName("name"); name != nil {
			return isPascalCaseName(name.Utf8Text(source))
		}
		return false
	case "function_expression":
		if name := n.ChildByFieldName("name"); name != nil {
			return isPascalCaseName(name.Utf8Text(source))
		}
		bound := tsBoundIdentifierName(n, source)
		return bound != "<func lit>" && isPascalCaseName(bound)
	case "arrow_function":
		bound := tsBoundIdentifierName(n, source)
		return bound != "<func lit>" && isPascalCaseName(bound)
	default:
		return false
	}
}
func reactIsHookCallee(call engine.Node, source []byte) bool {
	fn := call.ChildByFieldName("function")
	if fn == nil {
		return false
	}
	switch fn.Kind() {
	case "identifier":
		return reactHookNamePattern.MatchString(fn.Utf8Text(source))
	case "member_expression":
		obj := fn.ChildByFieldName("object")
		prop := fn.ChildByFieldName("property")
		if obj == nil || prop == nil || obj.Kind() != "identifier" || obj.Utf8Text(source) != "React" {
			return false
		}
		return reactHookNamePattern.MatchString(prop.Utf8Text(source))
	default:
		return false
	}
}

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

// reactJSXAttributeValueNode returns attr's value node -- attr's child 2,
// either a bare string or a jsx_expression -- or nil when attr has no
// value (e.g. a boolean shorthand attribute).
func reactJSXAttributeValueNode(attr engine.Node) engine.Node {
	if attr.ChildCount() < 3 {
		return nil
	}
	return attr.Child(2)
}
