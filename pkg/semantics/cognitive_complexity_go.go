package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// computeGoCognitiveComplexity discovers every scored Go function body under
// root and returns one FunctionCognitiveComplexity record per body (including
// zero scores), ordered by ascending location.start_byte then name.

type goCCTarget struct {
	node engine.Node
	name string
	kind string
}

// Iterative DFS into one result slice: avoids recursive slice-concat
// copies on large ASTs. Discovery order is irrelevant — callers sort by
// location.start_byte then name.

// goFuncLitName returns the short-decl/assignment LHS identifier when the
// literal is bound to exactly one identifier; otherwise "<func lit>".
func goFuncLitName(lit engine.Node, source []byte) string {
	parent := lit.Parent()
	if parent != nil && parent.Kind() == "expression_list" {
		parent = parent.Parent()
	}
	if parent == nil {
		return "<func lit>"
	}
	switch parent.Kind() {
	case "short_var_declaration", "assignment_statement":
		left := parent.ChildByFieldName("left")
		right := parent.ChildByFieldName("right")
		leftID := singleIdentifierName(left, source)
		if leftID == "" || !expressionListIsSingleNode(right, lit) {
			return "<func lit>"
		}
		return leftID
	default:
		return "<func lit>"
	}
}

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

// hybrid else if

// hybrid else

// countGoBooleanRuns implements the logical-sequence algorithm: +1 fundamental
// per maximal run of identical &&/|| operators in a flattened topmost chain.
