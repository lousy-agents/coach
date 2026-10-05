package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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

// checkAssignmentTarget inspects a single assignment target (one of an
// assignment_statement's possibly-multiple left-hand-side expressions) and
// emits a "mutates_input" Finding if it writes through a mutable parameter.
func (c *featureCollector) checkAssignmentTarget(target engine.Node, source []byte, funcName string, mutableParams map[string]paramMutKind) {
	if target == nil {
		return
	}

	switch target.Kind() {
	case "parenthesized_expression":
		inner := parenthesizedInner(target)
		if inner == nil {
			return
		}
		c.checkAssignmentTarget(inner, source, funcName, mutableParams)
	case "selector_expression":
		c.recordMutableSelectorOrIndex(target, source, funcName, mutableParams, paramMutPointer)
	case "index_expression":
		c.recordMutableSelectorOrIndex(target, source, funcName, mutableParams, paramMutCollection)
	case "unary_expression":
		c.recordMutableDeref(target, source, funcName, mutableParams)
	default:

	}
}
func identifiersInNodeField(n engine.Node, field string, source []byte) map[string]bool {
	child := n.ChildByFieldName(field)
	if child == nil {
		return nil
	}
	names := identSet{}
	names.collect(child, source)
	return names
}
