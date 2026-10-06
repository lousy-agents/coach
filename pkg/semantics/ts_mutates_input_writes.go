package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// checkMutatesInputAssignment emits a "mutates_input" Finding (Story 2) if
// n's left-hand side writes through a property or index rooted at some
// enclosing scope's identifier-bound parameter -- `p.x = ...`
// (member_expression) or `p[...] = ...` (subscript_expression), each with
// an "object" field resolving to a tracked parameter identifier. A plain
// identifier left-hand side (`p = other`) rebinds the local parameter
// variable rather than writing through it and is deliberately excluded.
// Evidence/Location are taken from the target (left-hand side) alone, not
// the whole assignment_expression: an assignment's right-hand side can be
// arbitrarily long (`p.x = someVeryLargeExpression()`), which would
// conflict with Evidence staying short, and would also diverge from the Go
// detector, whose evidence is likewise just the mutated selector/index
// target (e.g. cfg.Name), never including the assigned value.
func (c *tsFeatureCollector) checkMutatesInputAssignment(n engine.Node, source []byte, scopes []tsParamScope) {
	left := n.ChildByFieldName("left")
	base := tsMutationBase(left)
	if base != nil {
		c.recordMutatesInput(base, left, source, scopes)
		return
	}
	c.recordMutatesInputAssignmentTargets(left, source, scopes)
}

func (c *tsFeatureCollector) recordMutatesInputAssignmentTargets(n engine.Node, source []byte, scopes []tsParamScope) {
	if n == nil {
		return
	}
	if base := tsMutationBase(n); base != nil {
		c.recordMutatesInput(base, n, source, scopes)
		return
	}
	switch n.Kind() {
	case "parenthesized_expression":
		c.recordMutatesInputAssignmentTargets(tsWrappedExpressionInner(n), source, scopes)
	case "pair_pattern":
		c.recordMutatesInputAssignmentTargets(n.ChildByFieldName("value"), source, scopes)
	case "assignment_pattern":
		c.recordMutatesInputAssignmentTargets(n.ChildByFieldName("left"), source, scopes)
	case "rest_pattern":
		c.recordMutatesInputAssignmentTargets(n.ChildByFieldName("argument"), source, scopes)
	case "object_pattern", "array_pattern":
		count := n.ChildCount()
		for i := 0; i < count; i++ {
			c.recordMutatesInputAssignmentTargets(n.Child(i), source, scopes)
		}
	}
}

func (c *tsFeatureCollector) checkMutatesInputUpdate(n engine.Node, source []byte, scopes []tsParamScope) {
	target := n.ChildByFieldName("argument")
	base := tsMutationBase(target)
	if base == nil {
		return
	}
	c.recordMutatesInput(base, target, source, scopes)
}

// checkMutatesInputDelete emits a "mutates_input" Finding (Story 2) if n
// (a unary_expression) is a `delete` of a property or index rooted at some
// enclosing scope's identifier-bound parameter (`delete p.x`,
// `delete p['x']`). Unlike checkMutatesInputAssignment/checkMutatesInputCall,
// Evidence/Location are taken from n itself (the whole "delete ..."
// expression) rather than just the target: a delete unary_expression has no
// extra unbounded content beyond its "delete" keyword and target argument,
// so it is already short and bounded, and keeping the keyword makes the
// evidence self-explanatory as a deletion rather than a read.
func (c *tsFeatureCollector) checkMutatesInputDelete(n engine.Node, source []byte, scopes []tsParamScope) {
	operator := n.ChildByFieldName("operator")
	if operator == nil || operator.Utf8Text(source) != "delete" {
		return
	}
	base := tsMutationBase(n.ChildByFieldName("argument"))
	if base == nil {
		return
	}
	c.recordMutatesInput(base, n, source, scopes)
}
