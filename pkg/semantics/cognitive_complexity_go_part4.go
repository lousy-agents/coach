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
func isGoDirectRecursion(call engine.Node, source []byte, funcName string) bool {
	if funcName == "" {
		return false
	}
	fn := call.ChildByFieldName("function")
	if fn == nil || fn.Kind() != "identifier" {
		return false
	}
	return fn.Utf8Text(source) == funcName
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

// walkIf scores a leading if plus its else-if/else chain with one shared
// nesting increment (Go else-if normalization + hybrid branches).
func (s *goCCScorer) walkIf(n engine.Node, depth int) {
	s.score += 1 + depth
	nest := depth + 1
	s.walk(n.ChildByFieldName("initializer"), nest)
	s.walk(n.ChildByFieldName("condition"), nest)
	s.walk(n.ChildByFieldName("consequence"), nest)

	alt := n.ChildByFieldName("alternative")
	for alt != nil && alt.Kind() == "if_statement" {
		s.score++
		s.walk(alt.ChildByFieldName("initializer"), nest)
		s.walk(alt.ChildByFieldName("condition"), nest)
		s.walk(alt.ChildByFieldName("consequence"), nest)
		alt = alt.ChildByFieldName("alternative")
	}
	if alt != nil {
		s.score++
		s.walk(alt, nest)
	}
}
func goBinaryOp(n engine.Node, source []byte) string {
	op := n.ChildByFieldName("operator")
	if op == nil {
		return ""
	}
	return op.Utf8Text(source)
}
