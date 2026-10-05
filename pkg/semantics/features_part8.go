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

// resultHasPointerType reports whether a function/method's result field
// contains a pointer_type anywhere in its subtree: as the result node
// itself (a single unnamed pointer return value), as a parameter_list's
// parameter_declaration type (multiple and/or named return values), or
// nested inside a composite type such as a slice, map value, or channel
// element (e.g. []*T, map[string]*T, chan *T). A full descendant search is
// used rather than checking only the direct result/type node, since Go
// permits pointer_type at any depth within a composite result type.
func resultHasPointerType(result engine.Node) bool {
	if result.Kind() == "pointer_type" {
		return true
	}
	count := result.ChildCount()
	for i := 0; i < count; i++ {
		if resultHasPointerType(result.Child(i)) {
			return true
		}
	}
	return false
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
