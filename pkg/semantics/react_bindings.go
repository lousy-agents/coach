package semantics

import "github.com/lousy-agents/coach/pkg/semantics/internal/engine"

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

// reactIsUseClientLiteral reports whether str's raw source text is exactly
// "use client" or 'use client'. tsStringLiteralText is not reused here
// because it delegates to strconv.Unquote, which rejects JS single-quoted
// string literals (a Go-only quoting rule), silently dropping the
// single-quoted spelling of the directive.
func reactIsUseClientLiteral(str engine.Node, source []byte) bool {
	text := str.Utf8Text(source)
	if len(text) < 2 {
		return false
	}
	quote := text[0]
	if (quote != '"' && quote != '\'') || text[len(text)-1] != quote {
		return false
	}
	return text[1:len(text)-1] == "use client"
}

func reactFirstStringChild(n engine.Node) engine.Node {
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		if c := n.Child(i); c.Kind() == "string" {
			return c
		}
	}
	return nil
}
