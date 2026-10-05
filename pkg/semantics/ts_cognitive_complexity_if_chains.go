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
			s.score++ // hybrid else if
			s.walk(alt.ChildByFieldName("condition"), nest, false)
			s.walk(alt.ChildByFieldName("consequence"), nest, false)
			alt = alt.ChildByFieldName("alternative")
			continue
		}
		s.score++ // hybrid else
		s.walk(alt, nest, false)
		break
	}
}

func tsElseClauseInner(elseClause engine.Node) engine.Node {
	if elseClause == nil {
		return nil
	}
	count := elseClause.ChildCount()
	for i := 0; i < count; i++ {
		c := elseClause.Child(i)
		if c.Kind() != "else" {
			return c
		}
	}
	return nil
}
