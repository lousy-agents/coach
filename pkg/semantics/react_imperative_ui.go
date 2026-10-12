package semantics

import (
	"sort"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// reactImperativeAPINames is the closed set of imperative DOM/UI APIs
// recorded; any other callee/property name is ignored.
var reactImperativeAPINames = map[string]bool{
	"getElementById":   true,
	"querySelector":    true,
	"querySelectorAll": true,
	"focus":            true,
	"blur":             true,
	"scrollIntoView":   true,
}

// reactExtractImperativeUI collects one ReactImperativeUICall per
// call_expression in body's scan set whose resolved API name (the callee
// identifier, or a member_expression callee's property -- covering both
// `.` and `?.` uniformly, since optional chaining has no distinct node
// kind) is in the closed reactImperativeAPINames set, ordered by the call's
// own start_byte.
func reactExtractImperativeUI(body engine.Node, source []byte) []ReactImperativeUICall {
	var calls []engine.Node
	reactWalkScope(body, source, func(n engine.Node) {
		if n.Kind() != "call_expression" {
			return
		}
		if reactImperativeAPI(n, source) != "" {
			calls = append(calls, n)
		}
	})
	sort.SliceStable(calls, func(i, j int) bool {
		return calls[i].StartByte() < calls[j].StartByte()
	})

	out := make([]ReactImperativeUICall, 0, len(calls))
	for _, c := range calls {
		out = append(out, ReactImperativeUICall{
			API:      reactImperativeAPI(c, source),
			Location: locationFromNode(c),
		})
	}
	return out
}

func reactImperativeAPI(call engine.Node, source []byte) string {
	fn := call.ChildByFieldName("function")
	if fn == nil {
		return ""
	}
	var api string
	switch fn.Kind() {
	case "member_expression":
		prop := fn.ChildByFieldName("property")
		if prop == nil {
			return ""
		}
		api = prop.Utf8Text(source)
	case "identifier":
		api = fn.Utf8Text(source)
	default:
		return ""
	}
	if reactImperativeAPINames[api] {
		return api
	}
	return ""
}
