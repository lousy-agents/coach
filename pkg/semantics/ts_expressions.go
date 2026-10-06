package semantics

import (
	"strconv"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func tsWrappedExpressionInner(expr engine.Node) engine.Node {
	for _, field := range []string{"expression", "operand", "argument"} {
		if child := expr.ChildByFieldName(field); child != nil {
			return child
		}
	}
	count := expr.ChildCount()
	for i := 0; i < count; i++ {
		child := expr.Child(i)
		switch child.Kind() {
		case "(", ")", "!":
			continue
		default:
			return child
		}
	}
	return nil
}

func unwrapTSParen(n engine.Node) engine.Node {
	for n != nil && (n.Kind() == "parenthesized_expression" || n.Kind() == "non_null_expression") {
		inner := tsWrappedExpressionInner(n)
		if inner == nil {
			return n
		}
		n = inner
	}
	return n
}

func tsBinaryOp(n engine.Node) string {
	op := n.ChildByFieldName("operator")
	if op == nil {
		return ""
	}
	// TS operator tokens use the operator text as Kind (e.g. "&&", "||").
	return op.Kind()
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
