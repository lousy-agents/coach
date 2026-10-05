package semantics

import (
	"strconv"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// tsMutationBase resolves expr (a candidate mutation target/argument) down
// to the root identifier it is ultimately rooted at, when expr is a
// member_expression or subscript_expression -- either directly (`p.x`,
// `p[...]`) or through a chain of nested member_expression/
// subscript_expression "object" fields (`p.x.y`, `p.items[0].name`) -- or
// nil for any other shape, including a bare identifier (handled
// separately, since a bare identifier as an assignment's left-hand side is
// a rebind, not a write-through) and a chain that bottoms out in something
// other than a plain identifier (e.g. `f().x`), which is not resolved to a
// root.
func tsMutationBase(expr engine.Node) engine.Node {
	if expr == nil {
		return nil
	}
	if expr.Kind() != "member_expression" && expr.Kind() != "subscript_expression" {
		return nil
	}
	return tsResolveRootIdentifier(expr.ChildByFieldName("object"))
}

// tsResolveRootIdentifier walks a chain of nested member_expression/
// subscript_expression "object" fields, starting at expr, until it reaches
// a plain identifier -- the root -- or determines there is no such root
// (e.g. the chain bottoms out in a call_expression like `f().x`), in which
// case it returns nil. Used by both tsMutationBase (assignment/delete
// targets) and checkMutatesInputCall (method-call receivers) so nested
// mutation targets/receivers rooted at a tracked parameter (`p.x.y = 1`,
// `p.items.push(1)`) are resolved the same way.
func tsResolveRootIdentifier(expr engine.Node) engine.Node {
	for expr != nil {
		switch expr.Kind() {
		case "identifier":
			return expr
		case "member_expression", "subscript_expression":
			expr = expr.ChildByFieldName("object")
		case "parenthesized_expression", "non_null_expression":
			expr = tsWrappedExpressionInner(expr)
		default:
			return nil
		}
	}
	return nil
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
func tsStringLiteralText(n engine.Node, source []byte) (string, bool) {
	if n == nil || n.Kind() != "string" {
		return "", false
	}
	value, err := strconv.Unquote(n.Utf8Text(source))
	if err != nil {
		return "", false
	}
	return value, true
}

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
func (c *tsFeatureCollector) checkMutatesInputUpdate(n engine.Node, source []byte, scopes []tsParamScope) {
	target := n.ChildByFieldName("argument")
	base := tsMutationBase(target)
	if base == nil {
		return
	}
	c.recordMutatesInput(base, target, source, scopes)
}
