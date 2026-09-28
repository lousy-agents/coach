package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func reactScopeInvokesHook(body engine.Node, source []byte) bool {
	found := false
	reactWalkScope(body, source, func(n engine.Node) {
		if found || n.Kind() != "call_expression" {
			return
		}
		if reactIsHookCallee(n, source) {
			found = true
		}
	})
	return found
}
func reactExportStatements(root engine.Node) []engine.Node {
	var out []engine.Node
	count := root.ChildCount()
	for i := 0; i < count; i++ {
		child := root.Child(i)
		if child.Kind() == "export_statement" {
			out = append(out, child)
		}
	}
	return out
}
func reactFirstArgumentNode(argsNode engine.Node) engine.Node {
	count := argsNode.ChildCount()
	for i := 0; i < count; i++ {
		c := argsNode.Child(i)
		switch c.Kind() {
		case "(", ")", ",":
			continue
		default:
			return c
		}
	}
	return nil
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

// collectModuleTopLevelBindings maps every top-level function declaration
// and plain-identifier-bound const/let/var name to its declaration/
// initializer node, for resolving `export default Name` and
// `export { Name }` against a same-module binding. A top-level statement
// wrapped in `export ...` is also registered under its own name (e.g.
// `export const C = ...` registers "C"). A binding registered this way can
// still be referenced by another export form in the same module (e.g.
// `export const Page = ...` plus `export default Page;`); computeReactComponents
// dedupes the resulting candidates by resolved function node span so such
// re-exports produce one record, not two.
func collectModuleTopLevelBindings(root engine.Node, source []byte) map[string]engine.Node {
	bindings := map[string]engine.Node{}
	count := root.ChildCount()
	for i := 0; i < count; i++ {
		for name, node := range reactModuleBindingsFrom(root.Child(i), source) {
			bindings[name] = node
		}
	}
	return bindings
}
