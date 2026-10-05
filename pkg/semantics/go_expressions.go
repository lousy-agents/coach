package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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

func unwrapGoParen(n engine.Node) engine.Node {
	for n != nil && n.Kind() == "parenthesized_expression" {
		inner := parenthesizedInner(n)
		if inner == nil {
			return n
		}
		n = inner
	}
	return n
}

func goBinaryOp(n engine.Node, source []byte) string {
	op := n.ChildByFieldName("operator")
	if op == nil {
		return ""
	}
	return op.Utf8Text(source)
}
