package semantics

import (
	"sort"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// computeTSCognitiveComplexity discovers every scored TS/TSX function body
// under root and returns one record per body plus a parallel topLevel slice
// (true when the body is not lexically nested inside another scored body).
func computeTSCognitiveComplexity(root engine.Node, source []byte) ([]FunctionCognitiveComplexity, []bool) {
	if root == nil {
		return nil, nil
	}
	targets := collectTSCCTargets(root, source)
	if len(targets) == 0 {
		return nil, nil
	}
	records := make([]FunctionCognitiveComplexity, 0, len(targets))
	topLevel := make([]bool, 0, len(targets))
	for _, t := range targets {
		body := tsCCBody(t.node)
		score := 0
		if body != nil {
			s := &tsCCScorer{source: source, funcName: t.name}
			s.walk(body, 0, false)
			score = s.score
		}
		records = append(records, FunctionCognitiveComplexity{
			Name:     t.name,
			Kind:     t.kind,
			Location: locationFromNode(t.node),
			Score:    score,
		})
		topLevel = append(topLevel, t.topLevel)
	}
	// Stable order: start_byte then name (matches Go path / JSON contract).
	type pair struct {
		r FunctionCognitiveComplexity
		t bool
	}
	pairs := make([]pair, len(records))
	for i := range records {
		pairs[i] = pair{records[i], topLevel[i]}
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i].r.Location.StartByte != pairs[j].r.Location.StartByte {
			return pairs[i].r.Location.StartByte < pairs[j].r.Location.StartByte
		}
		return pairs[i].r.Name < pairs[j].r.Name
	})
	for i := range pairs {
		records[i] = pairs[i].r
		topLevel[i] = pairs[i].t
	}
	return records, topLevel
}

type tsCCTarget struct {
	node     engine.Node
	name     string
	kind     string
	topLevel bool
}

func isTSCCScoredKind(kind string) bool {
	switch kind {
	case "function_declaration", "generator_function_declaration",
		"function_expression", "generator_function",
		"arrow_function", "method_definition":
		return true
	default:
		return false
	}
}

func tsCCKind(nodeKind string) string {
	switch nodeKind {
	case "function_declaration", "generator_function_declaration":
		return "function"
	case "method_definition":
		return "method"
	case "function_expression", "generator_function":
		return "func_lit"
	case "arrow_function":
		return "arrow"
	default:
		return ""
	}
}

func tsCCBody(n engine.Node) engine.Node {
	if n == nil {
		return nil
	}
	return n.ChildByFieldName("body")
}

// tsIsNestedInScoredBody reports whether n sits lexically inside another
// scored function/method/arrow/func_lit (class bodies do not count).
func tsIsNestedInScoredBody(n engine.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if isTSCCScoredKind(p.Kind()) {
			return true
		}
	}
	return false
}

func collectTSCCTargets(root engine.Node, source []byte) []tsCCTarget {
	if root == nil {
		return nil
	}
	// Iterative DFS into one result slice: avoids recursive slice-concat
	// copies on large ASTs. Discovery order is irrelevant — callers sort by
	// location.start_byte then name.
	var out []tsCCTarget
	stack := []engine.Node{root}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if isTSCCScoredKind(n.Kind()) {
			// Overload signatures and abstract methods have no body — skip.
			if body := tsCCBody(n); body != nil {
				out = append(out, tsCCTarget{
					node:     n,
					name:     tsCCName(n, source),
					kind:     tsCCKind(n.Kind()),
					topLevel: !tsIsNestedInScoredBody(n),
				})
			}
		}
		count := n.ChildCount()
		for i := count - 1; i >= 0; i-- {
			stack = append(stack, n.Child(i))
		}
	}
	return out
}

func tsCCName(n engine.Node, source []byte) string {
	switch n.Kind() {
	case "function_declaration", "generator_function_declaration", "method_definition":
		if name := n.ChildByFieldName("name"); name != nil {
			return name.Utf8Text(source)
		}
		return "<func lit>"
	case "function_expression", "generator_function":
		// Prefer the expression's own name identifier when present.
		if name := n.ChildByFieldName("name"); name != nil {
			return name.Utf8Text(source)
		}
		return tsBoundIdentifierName(n, source)
	case "arrow_function":
		return tsBoundIdentifierName(n, source)
	default:
		return "<func lit>"
	}
}

// tsBoundIdentifierName returns the single identifier a lit/arrow is bound to
// via variable_declarator or assignment_expression; otherwise "<func lit>".
func tsBoundIdentifierName(lit engine.Node, source []byte) string {
	parent := lit.Parent()
	if parent == nil {
		return "<func lit>"
	}
	switch parent.Kind() {
	case "variable_declarator":
		name := parent.ChildByFieldName("name")
		value := parent.ChildByFieldName("value")
		if name == nil || name.Kind() != "identifier" || value == nil || !sameNodeSpan(value, lit) {
			return "<func lit>"
		}
		return name.Utf8Text(source)
	case "assignment_expression":
		left := parent.ChildByFieldName("left")
		right := parent.ChildByFieldName("right")
		if left == nil || left.Kind() != "identifier" || right == nil || !sameNodeSpan(right, lit) {
			return "<func lit>"
		}
		return left.Utf8Text(source)
	default:
		return "<func lit>"
	}
}

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

func tsHasLabel(n engine.Node) bool {
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		if n.Child(i).Kind() == "statement_identifier" {
			return true
		}
	}
	return false
}

func isTSDirectRecursion(call engine.Node, source []byte, funcName string) bool {
	if funcName == "" || funcName == "<func lit>" {
		return false
	}
	fn := call.ChildByFieldName("function")
	if fn == nil || fn.Kind() != "identifier" {
		return false
	}
	return fn.Utf8Text(source) == funcName
}

func tsBinaryOp(n engine.Node) string {
	op := n.ChildByFieldName("operator")
	if op == nil {
		return ""
	}
	// TS operator tokens use the operator text as Kind (e.g. "&&", "||").
	return op.Kind()
}

func isTSBooleanBinary(n engine.Node) bool {
	if n == nil || n.Kind() != "binary_expression" {
		return false
	}
	op := tsBinaryOp(n)
	return op == "&&" || op == "||"
}

func unwrapTSParen(n engine.Node) engine.Node {
	for n != nil && (n.Kind() == "parenthesized_expression" || n.Kind() == "non_null_expression") {
		inner := tsWrappedExpressionInner(n)
		if inner == nil {
			return n
		}
		n = inner
	}
	return n
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

func flattenTSBooleanOps(n engine.Node) []string {
	n = unwrapTSParen(n)
	if !isTSBooleanBinary(n) {
		return nil
	}
	op := tsBinaryOp(n)
	left := flattenTSBooleanOps(n.ChildByFieldName("left"))
	right := flattenTSBooleanOps(n.ChildByFieldName("right"))
	out := make([]string, 0, len(left)+1+len(right))
	out = append(out, left...)
	out = append(out, op)
	out = append(out, right...)
	return out
}
