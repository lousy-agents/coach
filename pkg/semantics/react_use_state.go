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
