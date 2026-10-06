package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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

func reactModuleBindingsFrom(n engine.Node, source []byte) map[string]engine.Node {
	switch n.Kind() {
	case "function_declaration":
		name := n.ChildByFieldName("name")
		if name == nil {
			return nil
		}
		return map[string]engine.Node{name.Utf8Text(source): n}
	case "lexical_declaration", "variable_declaration":
		return reactDeclaratorBindingMap(n, source)
	case "export_statement":
		decl := n.ChildByFieldName("declaration")
		if decl == nil {
			return nil
		}
		return reactModuleBindingsFrom(decl, source)
	default:
		return nil
	}
}

func reactDeclaratorBindingMap(n engine.Node, source []byte) map[string]engine.Node {
	out := map[string]engine.Node{}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		d := n.Child(i)
		if d.Kind() != "variable_declarator" {
			continue
		}
		name := d.ChildByFieldName("name")
		value := d.ChildByFieldName("value")
		if name == nil || name.Kind() != "identifier" || value == nil {
			continue
		}
		out[name.Utf8Text(source)] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
