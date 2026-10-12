package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func countTSBooleanRuns(n engine.Node) int {
	ops := flattenTSBooleanOps(n)
	if len(ops) == 0 {
		return 0
	}
	runs := 1
	for i := 1; i < len(ops); i++ {
		if ops[i] != ops[i-1] {
			runs++
		}
	}
	return runs
}

func flattenTSBooleanOps(n engine.Node) []string {
	n = unwrapTSParen(n)
	if !isTSBooleanBinary(n) {
		return nil
	}
	op := tsBinaryOp(n)
	left := flattenTSBooleanOps(n.ChildByFieldName("left"))
	right := flattenTSBooleanOps(n.ChildByFieldName("right"))
	out := make([]string, 0, len(left)+1+len(right))
	out = append(out, left...)
	out = append(out, op)
	out = append(out, right...)
	return out
}

func isTSBooleanBinary(n engine.Node) bool {
	if n == nil || n.Kind() != "binary_expression" {
		return false
	}
	op := tsBinaryOp(n)
	return op == "&&" || op == "||"
}
