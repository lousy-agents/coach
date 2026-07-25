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
	var targets []goCCTarget
	collectGoCCTargets(root, source, &targets)
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

// applyCognitiveComplexityAggregates sets max (all records) and sum on metrics.
// When topLevel is nil, sum uses the Go convention (kind function|method only).
// When topLevel is non-nil, it must match records length; sum includes only
// indices where topLevel[i] is true (TS/TSX lexical top-level rule).
func applyCognitiveComplexityAggregates(metrics *StructuralMetrics, records []FunctionCognitiveComplexity, topLevel []bool) {
	if metrics == nil {
		return
	}
	max, sum := 0, 0
	for i, r := range records {
		if r.Score > max {
			max = r.Score
		}
		include := r.Kind == "function" || r.Kind == "method"
		if topLevel != nil {
			include = i < len(topLevel) && topLevel[i]
		}
		if include {
			sum += r.Score
		}
	}
	metrics.MaxCognitiveComplexity = max
	metrics.SumCognitiveComplexity = sum
}

type goCCTarget struct {
	node engine.Node
	name string
	kind string
}

func collectGoCCTargets(n engine.Node, source []byte, out *[]goCCTarget) {
	if n == nil {
		return
	}
	switch n.Kind() {
	case "function_declaration":
		*out = append(*out, goCCTarget{node: n, name: goDeclName(n, source), kind: "function"})
	case "method_declaration":
		*out = append(*out, goCCTarget{node: n, name: goDeclName(n, source), kind: "method"})
	case "func_literal":
		*out = append(*out, goCCTarget{node: n, name: goFuncLitName(n, source), kind: "func_lit"})
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		collectGoCCTargets(n.Child(i), source, out)
	}
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

func sameNodeSpan(a, b engine.Node) bool {
	return a.StartByte() == b.StartByte() && a.EndByte() == b.EndByte() && a.Kind() == b.Kind()
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

// computeTSCognitiveComplexity discovers every scored TS/TSX function body
// under root and returns one record per body plus a parallel topLevel slice
// (true when the body is not lexically nested inside another scored body).
func computeTSCognitiveComplexity(root engine.Node, source []byte) ([]FunctionCognitiveComplexity, []bool) {
	if root == nil {
		return nil, nil
	}
	var targets []tsCCTarget
	collectTSCCTargets(root, source, &targets)
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

func collectTSCCTargets(n engine.Node, source []byte, out *[]tsCCTarget) {
	if n == nil {
		return
	}
	if isTSCCScoredKind(n.Kind()) {
		// Overload signatures and abstract methods have no body — skip.
		if body := tsCCBody(n); body != nil {
			kind := tsCCKind(n.Kind())
			*out = append(*out, tsCCTarget{
				node:     n,
				name:     tsCCName(n, source),
				kind:     kind,
				topLevel: !tsIsNestedInScoredBody(n),
			})
		}
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		collectTSCCTargets(n.Child(i), source, out)
	}
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
