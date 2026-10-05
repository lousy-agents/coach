package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func tsHasLabel(n engine.Node) bool {
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		if n.Child(i).Kind() == "statement_identifier" {
			return true
		}
	}
	return false
}
func isTSDirectRecursion(call engine.Node, source []byte, funcName string) bool {
	if funcName == "" || funcName == "<func lit>" {
		return false
	}
	fn := call.ChildByFieldName("function")
	if fn == nil || fn.Kind() != "identifier" {
		return false
	}
	return fn.Utf8Text(source) == funcName
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

// tsIsNestedInScoredBody reports whether n sits lexically inside another
// scored function/method/arrow/func_lit (class bodies do not count).
func tsIsNestedInScoredBody(n engine.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if isTSCCScoredKind(p.Kind()) {
			return true
		}
	}
	return false
}
func tsBinaryOp(n engine.Node) string {
	op := n.ChildByFieldName("operator")
	if op == nil {
		return ""
	}

	return op.Kind()
}
func isTSBooleanBinary(n engine.Node) bool {
	if n == nil || n.Kind() != "binary_expression" {
		return false
	}
	op := tsBinaryOp(n)
	return op == "&&" || op == "||"
}
