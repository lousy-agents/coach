package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func isTopmostGoBooleanBinary(n engine.Node, source []byte) bool {
	p := n.Parent()
	for p != nil && p.Kind() == "parenthesized_expression" {
		p = p.Parent()
	}
	return p == nil || !isGoBooleanBinary(p, source)
}
func (s *goCCScorer) walkStructural(n engine.Node, depth int) {
	s.score += 1 + depth
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		s.walk(n.Child(i), depth+1)
	}
}
func goDeclName(decl engine.Node, source []byte) string {
	name := decl.ChildByFieldName("name")
	if name == nil {
		return ""
	}
	return name.Utf8Text(source)
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
