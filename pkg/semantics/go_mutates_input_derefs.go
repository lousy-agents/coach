package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func (c *featureCollector) recordMutableDeref(target engine.Node, source []byte, funcName string, mutableParams map[string]paramMutKind) {
	op := target.ChildByFieldName("operator")
	if op == nil || op.Utf8Text(source) != "*" {
		return
	}
	operand := target.ChildByFieldName("operand")
	if operand == nil || operand.Kind() != "identifier" {
		return
	}
	paramName := operand.Utf8Text(source)
	if mutableParams[paramName] != paramMutPointer {
		return
	}
	c.recordMutatesInput(funcName, paramName, target, source)
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
