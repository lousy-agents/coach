package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

type tsCCScorer struct {
	source   []byte
	funcName string
	score    int
}

// walk scores n. inBoolChain is true when n is already part of a boolean
// &&/|| chain whose runs were charged at the chain root — nested boolean
// binaries must not re-charge (gotreesitter Parent() is unreliable here).
func (s *tsCCScorer) walk(n engine.Node, depth int, inBoolChain bool) {
	if n == nil {
		return
	}

	switch n.Kind() {
	case "if_statement":
		s.walkIf(n, depth)
		return
	case "for_statement", "for_in_statement", "while_statement", "do_statement":
		s.walkStructural(n, depth)
		return
	case "switch_statement":
		s.walkStructural(n, depth)
		return
	case "ternary_expression":
		s.walkStructural(n, depth)
		return
	case "catch_clause":
		s.walkStructural(n, depth)
		return
	case "function_declaration", "generator_function_declaration",
		"function_expression", "generator_function",
		"arrow_function", "method_definition":
		// Nested scored body: +0 structural; raises nesting for enclosing walk.
		s.walk(tsCCBody(n), depth+1, false)
		return
	case "binary_expression":
		if isTSBooleanBinary(n) {
			if !inBoolChain {
				s.score += countTSBooleanRuns(n)
			}
			s.walkTSBooleanOperand(n.ChildByFieldName("left"), depth)
			s.walkTSBooleanOperand(n.ChildByFieldName("right"), depth)
			return
		}
	case "break_statement", "continue_statement":
		if tsHasLabel(n) {
			s.score++
		}
		return
	case "call_expression":
		if isTSDirectRecursion(n, s.source, s.funcName) {
			s.score++
		}
	}

	count := n.ChildCount()
	for i := 0; i < count; i++ {
		s.walk(n.Child(i), depth, false)
	}
}

func (s *tsCCScorer) walkStructural(n engine.Node, depth int) {
	s.score += 1 + depth
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		s.walk(n.Child(i), depth+1, false)
	}
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
