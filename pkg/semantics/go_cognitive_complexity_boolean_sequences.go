package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// countGoBooleanRuns implements the logical-sequence algorithm: +1 fundamental
// per maximal run of identical &&/|| operators in a flattened topmost chain.
func countGoBooleanRuns(n engine.Node, source []byte) int {
	ops := flattenGoBooleanOps(n, source)
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

func flattenGoBooleanOps(n engine.Node, source []byte) []string {
	n = unwrapGoParen(n)
	if !isGoBooleanBinary(n, source) {
		return nil
	}
	op := goBinaryOp(n, source)
	left := flattenGoBooleanOps(n.ChildByFieldName("left"), source)
	right := flattenGoBooleanOps(n.ChildByFieldName("right"), source)
	out := make([]string, 0, len(left)+1+len(right))
	out = append(out, left...)
	out = append(out, op)
	out = append(out, right...)
	return out
}

func isGoBooleanBinary(n engine.Node, source []byte) bool {
	if n == nil || n.Kind() != "binary_expression" {
		return false
	}
	op := goBinaryOp(n, source)
	return op == "&&" || op == "||"
}

func isTopmostGoBooleanBinary(n engine.Node, source []byte) bool {
	p := n.Parent()
	for p != nil && p.Kind() == "parenthesized_expression" {
		p = p.Parent()
	}
	return p == nil || !isGoBooleanBinary(p, source)
}
