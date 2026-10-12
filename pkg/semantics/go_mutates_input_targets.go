package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func (c *featureCollector) checkAssignmentTargets(left engine.Node, source []byte, funcName string, mutableParams map[string]paramMutKind) {
	if left == nil {
		return
	}
	targetCount := left.ChildCount()
	for i := 0; i < targetCount; i++ {
		c.checkAssignmentTarget(left.Child(i), source, funcName, mutableParams)
	}
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
		// Plain identifier targets (cfg = other) rebind the local
		// parameter variable rather than writing through it, and any
		// other target kind is out of scope for this detector.
	}
}

func updateStatementTarget(n engine.Node) engine.Node {
	if n == nil {
		return nil
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		child := n.Child(i)
		switch child.Kind() {
		case "selector_expression", "index_expression", "unary_expression":
			return child
		}
	}
	return nil
}
