package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func parameterNames(params engine.Node, source []byte) map[string]bool {
	names := map[string]bool{}
	if params == nil {
		return names
	}
	count := params.ChildCount()
	for i := 0; i < count; i++ {
		identSet(names).collectParams(params.Child(i), source)
	}
	return names
}
func parenthesizedInner(paren engine.Node) engine.Node {
	count := paren.ChildCount()
	for i := 0; i < count; i++ {
		child := paren.Child(i)
		if child.Kind() != "(" && child.Kind() != ")" {
			return child
		}
	}
	return nil
}

// shadowLocalDeclarations returns a copy of outer with any entries shadowed by
// local declarations or direct parameter rebindings in n removed. It is
// applied while walking child nodes in source order so a short variable
// declaration such as `cfg := &Config{}` or a parameter rebind such as
// `cfg = other` shadows an outer parameter only for subsequent source nodes.
func shadowLocalDeclarations(outer map[string]paramMutKind, n engine.Node, source []byte) map[string]paramMutKind {
	if len(outer) == 0 || n == nil {
		return outer
	}

	var names map[string]bool
	switch n.Kind() {
	case "short_var_declaration":
		names = identifiersInNodeField(n, "left", source)
	case "assignment_statement":
		names = reboundIdentifiersInAssignment(n, source)
	case "for_clause":
		names = identifiersDeclaredInSubtree(n, source)
	case "range_clause":
		names = identifiersInNodeField(n, "left", source)
	case "type_switch_header", "type_switch_guard":
		names = typeSwitchGuardIdentifiers(n, source)
	case "var_declaration":
		names = identifiersInVarDeclaration(n, source)
	default:
		return outer
	}
	return shadowNames(outer, names)
}
