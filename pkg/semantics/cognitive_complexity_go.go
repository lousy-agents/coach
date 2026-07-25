package semantics

import (
	"sort"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// computeGoCognitiveComplexity discovers every scored Go function body under
// root and returns one FunctionCognitiveComplexity record per body (including
// zero scores), ordered by ascending location.start_byte then name.
func computeGoCognitiveComplexity(root engine.Node, source []byte) []FunctionCognitiveComplexity {
	if root == nil {
		return nil
	}
	targets := collectGoCCTargets(root, source)
	if len(targets) == 0 {
		return nil
	}
	records := make([]FunctionCognitiveComplexity, 0, len(targets))
	for _, t := range targets {
		body := t.node.ChildByFieldName("body")
		score := 0
		if body != nil {
			s := &goCCScorer{source: source, funcName: t.name}
			s.walk(body, 0)
			score = s.score
		}
		records = append(records, FunctionCognitiveComplexity{
			Name:     t.name,
			Kind:     t.kind,
			Location: locationFromNode(t.node),
			Score:    score,
		})
	}
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].Location.StartByte != records[j].Location.StartByte {
			return records[i].Location.StartByte < records[j].Location.StartByte
		}
		return records[i].Name < records[j].Name
	})
	return records
}

type goCCTarget struct {
	node engine.Node
	name string
	kind string
}

func collectGoCCTargets(root engine.Node, source []byte) []goCCTarget {
	if root == nil {
		return nil
	}
	// Iterative DFS into one result slice: avoids recursive slice-concat
	// copies on large ASTs. Discovery order is irrelevant — callers sort by
	// location.start_byte then name.
	var out []goCCTarget
	stack := []engine.Node{root}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch n.Kind() {
		case "function_declaration":
			out = append(out, goCCTarget{node: n, name: goDeclName(n, source), kind: "function"})
		case "method_declaration":
			out = append(out, goCCTarget{node: n, name: goDeclName(n, source), kind: "method"})
		case "func_literal":
			out = append(out, goCCTarget{node: n, name: goFuncLitName(n, source), kind: "func_lit"})
		}
		count := n.ChildCount()
		for i := count - 1; i >= 0; i-- {
			stack = append(stack, n.Child(i))
		}
	}
	return out
}

func goDeclName(decl engine.Node, source []byte) string {
	name := decl.ChildByFieldName("name")
	if name == nil {
		return ""
	}
	return name.Utf8Text(source)
}

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

func singleIdentifierName(list engine.Node, source []byte) string {
	if list == nil {
		return ""
	}
	var id string
	count := list.ChildCount()
	for i := 0; i < count; i++ {
		child := list.Child(i)
		switch child.Kind() {
		case ",":
			continue
		case "identifier":
			if id != "" {
				return ""
			}
			id = child.Utf8Text(source)
		default:
			return ""
		}
	}
	return id
}

func expressionListIsSingleNode(list engine.Node, want engine.Node) bool {
	if list == nil || want == nil {
		return false
	}
	var found engine.Node
	count := list.ChildCount()
	for i := 0; i < count; i++ {
		child := list.Child(i)
		if child.Kind() == "," {
			continue
		}
		if found != nil {
			return false
		}
		found = child
	}
	return found != nil && sameNodeSpan(found, want)
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

func (s *goCCScorer) walkStructural(n engine.Node, depth int) {
	s.score += 1 + depth
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		s.walk(n.Child(i), depth+1)
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

func goHasLabel(n engine.Node) bool {
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		if n.Child(i).Kind() == "label_name" {
			return true
		}
	}
	return false
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

func isGoBooleanBinary(n engine.Node, source []byte) bool {
	if n == nil || n.Kind() != "binary_expression" {
		return false
	}
	op := goBinaryOp(n, source)
	return op == "&&" || op == "||"
}

func goBinaryOp(n engine.Node, source []byte) string {
	op := n.ChildByFieldName("operator")
	if op == nil {
		return ""
	}
	return op.Utf8Text(source)
}

func isTopmostGoBooleanBinary(n engine.Node, source []byte) bool {
	p := n.Parent()
	for p != nil && p.Kind() == "parenthesized_expression" {
		p = p.Parent()
	}
	return p == nil || !isGoBooleanBinary(p, source)
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
