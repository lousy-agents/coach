package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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

// shadowNames returns a copy of outer with any entries whose name appears in
// names removed.
func shadowNames(outer map[string]paramMutKind, names map[string]bool) map[string]paramMutKind {
	if len(outer) == 0 || len(names) == 0 {
		return outer
	}

	shadowed := make(map[string]paramMutKind, len(outer))
	for name, kind := range outer {
		if names[name] {
			continue
		}
		shadowed[name] = kind
	}
	return shadowed
}

// shadowParamTypes returns a copy of outer with any entries shadowed by
// literalParams (a func_literal's own "parameters" field) removed, so that
// a closure declaring its own parameter with the same name as an outer
// function's parameter (e.g. an outer `cfg` shadowed by a closure's own
// `func(cfg *Config){ ... }`) is never misattributed to the outer
// parameter: the closure's binding is a distinct variable per normal Go
// scoping, and this single-pass walk has no separate owner name to
// attribute the closure's own mutations to, so those are simply not
// reported rather than being reported against the wrong (outer) name.
// Outer parameters not redeclared by the literal are left untouched, since
// the walk is still inside their scope.
func shadowParamTypes(outer map[string]paramMutKind, literalParams engine.Node, source []byte) map[string]paramMutKind {
	ownParams := parameterNames(literalParams, source)
	if len(ownParams) == 0 {
		return outer
	}
	shadowed := make(map[string]paramMutKind, len(outer))
	for name, kind := range outer {
		if ownParams[name] {
			continue
		}
		shadowed[name] = kind
	}
	return shadowed
}

func reboundIdentifiersInAssignment(n engine.Node, source []byte) map[string]bool {
	left := n.ChildByFieldName("left")
	if left == nil {
		return nil
	}
	names := map[string]bool{}
	count := left.ChildCount()
	for i := 0; i < count; i++ {
		child := left.Child(i)
		if child.Kind() == "identifier" {
			names[child.Utf8Text(source)] = true
		}
	}
	return names
}
