package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// walkIf scores a leading if plus its else-if/else chain. TS wraps each else
// branch in an else_clause node whose non-else child is the alternative.
func (s *tsCCScorer) walkIf(n engine.Node, depth int) {
	s.score += 1 + depth
	nest := depth + 1
	s.walk(n.ChildByFieldName("condition"), nest, false)
	s.walk(n.ChildByFieldName("consequence"), nest, false)

	alt := n.ChildByFieldName("alternative")
	for alt != nil {
		if alt.Kind() == "else_clause" {
			alt = tsElseClauseInner(alt)
		}
		if alt == nil {
			break
		}
		if alt.Kind() == "if_statement" {
			s.score++
			s.walk(alt.ChildByFieldName("condition"), nest, false)
			s.walk(alt.ChildByFieldName("consequence"), nest, false)
			alt = alt.ChildByFieldName("alternative")
			continue
		}
		s.score++
		s.walk(alt, nest, false)
		break
	}
}
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

// walkTSBooleanOperand walks one side of a boolean binary after unwrapping
// parentheses. Nested &&/|| stay in the chain (inBoolChain=true); other
// operands are scored normally.
func (s *tsCCScorer) walkTSBooleanOperand(n engine.Node, depth int) {
	n = unwrapTSParen(n)
	if n == nil {
		return
	}
	if isTSBooleanBinary(n) {
		s.walk(n, depth, true)
		return
	}
	s.walk(n, depth, false)
}
func (s *tsCCScorer) walkStructural(n engine.Node, depth int) {
	s.score += 1 + depth
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		s.walk(n.Child(i), depth+1, false)
	}
}
