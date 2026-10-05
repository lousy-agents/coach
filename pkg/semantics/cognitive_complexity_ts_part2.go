package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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
