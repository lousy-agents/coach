package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// selectorBaseIdentifier resolves a selector_expression's operand down to
// the root identifier it ultimately reads, walking through any chain of
// nested selector_expression/index_expression operands (cfg.Sub.Name,
// cfg.Items[0].Name) and the parenthesized-unary-dereference case
// ((*cfg).Name, (*cfg.Sub).Name), iterating until it reaches a plain
// identifier -- the root -- or determines there is no such root (e.g. the
// chain bottoms out in a function call like f().Name), in which case base
// is nil.
//
// nested reports whether reaching that root required stepping through at
// least one selector_expression/index_expression hop beyond the "direct"
// shapes the acceptance criteria name explicitly: a bare identifier
// (cfg.Name, where operand IS the parameter) or a single parenthesized
// dereference of one ((*cfg).Name). Those two direct shapes report
// nested == false; anything requiring an additional hop (cfg.Sub.Name,
// cfg.Items[0]) reports nested == true. checkAssignmentTarget uses this to
// require an exact selector<->pointer / index<->collection kind match only
// for direct targets, where the acceptance criteria pin down the required
// parameter kind precisely -- a nested target's intermediate field/element
// type is not visible to this syntax-only detector, so it is checked more
// permissively (see checkAssignmentTarget's comment).
func selectorBaseIdentifier(operand engine.Node, source []byte) (base engine.Node, nested bool) {
	switch operand.Kind() {
	case "identifier":
		return operand, false
	case "parenthesized_expression":
		if inner := derefOperand(operand, source); inner != nil && inner.Kind() == "identifier" {
			return inner, false
		}
		if inner := parenthesizedInner(operand); inner != nil && inner.Kind() == "identifier" {
			return inner, false
		}
	}
	for operand != nil {
		switch operand.Kind() {
		case "identifier":
			return operand, true
		case "selector_expression", "index_expression":
			operand = operand.ChildByFieldName("operand")
		case "parenthesized_expression":
			if inner := derefOperand(operand, source); inner != nil {
				operand = inner
			} else {
				operand = parenthesizedInner(operand)
			}
		default:
			return nil, false
		}
	}
	return nil, false
}

// derefOperand returns the operand of the parenthesized unary "*"
// (dereference) expression nested directly inside a parenthesized_expression
// ((*cfg) -> cfg, (*cfg.Sub) -> cfg.Sub), or nil if paren does not wrap
// exactly that shape. A unary_expression's "operator" field must be checked
// by source text, not just by the presence of a unary_expression node: Go's
// grammar uses the same unary_expression node for "*", "&", "!", "-", "+",
// "^", and "<-", so a parenthesized non-dereference unary expression like
// (-x), (!x), or (&x) must not be mis-resolved as if it were (*x).
func derefOperand(paren engine.Node, source []byte) engine.Node {
	count := paren.ChildCount()
	for i := 0; i < count; i++ {
		child := paren.Child(i)
		if child.Kind() != "unary_expression" {
			continue
		}
		op := child.ChildByFieldName("operator")
		if op == nil || op.Utf8Text(source) != "*" {
			continue
		}
		if inner := child.ChildByFieldName("operand"); inner != nil {
			return inner
		}
	}
	return nil
}
