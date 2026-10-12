package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

type goCCScorer struct {
	source   []byte
	funcName string
	score    int
}

func (s *goCCScorer) walk(n engine.Node, depth int) {
	if n == nil {
		return
	}

	switch n.Kind() {
	case "if_statement":
		s.walkIf(n, depth)
		return
	case "for_statement":
		s.walkStructural(n, depth)
		return
	case "expression_switch_statement", "type_switch_statement", "select_statement":
		s.walkStructural(n, depth)
		return
	case "func_literal":
		// Nested lit: +0 structural; raises nesting for the enclosing walk.
		body := n.ChildByFieldName("body")
		s.walk(body, depth+1)
		return
	case "function_declaration", "method_declaration":
		body := n.ChildByFieldName("body")
		s.walk(body, depth+1)
		return
	case "binary_expression":
		if isGoBooleanBinary(n, s.source) && isTopmostGoBooleanBinary(n, s.source) {
			s.score += countGoBooleanRuns(n, s.source)
		}
	case "break_statement", "continue_statement":
		if goHasLabel(n) {
			s.score++
		}
		return
	case "goto_statement":
		s.score++
		return
	case "call_expression":
		if isGoDirectRecursion(n, s.source, s.funcName) {
			s.score++
		}
	}

	count := n.ChildCount()
	for i := 0; i < count; i++ {
		s.walk(n.Child(i), depth)
	}
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
		s.score++ // hybrid else if
		s.walk(alt.ChildByFieldName("initializer"), nest)
		s.walk(alt.ChildByFieldName("condition"), nest)
		s.walk(alt.ChildByFieldName("consequence"), nest)
		alt = alt.ChildByFieldName("alternative")
	}
	if alt != nil {
		s.score++ // hybrid else
		s.walk(alt, nest)
	}
}

func (s *goCCScorer) walkStructural(n engine.Node, depth int) {
	s.score += 1 + depth
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		s.walk(n.Child(i), depth+1)
	}
}
