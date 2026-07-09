package semantics

import (
	"fmt"
	"strconv"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// computeTSFeatures walks root exactly once, producing both the structural
// metrics and the tight_coupling/mutates_input findings for a TypeScript or
// TSX file (shared by both languageSpec entries: the walk matches on
// Node.Kind() strings alone, which every Node resolves against its own
// tree's language, so it needs no grammar-specific Query the way import
// extraction does). TypeSwitches and Selects have no TypeScript analog and
// are always 0 (D2).
func computeTSFeatures(root engine.Node, source []byte) (StructuralMetrics, []Finding) {
	c := &tsFeatureCollector{}
	c.walk(root, source, 0, false, false, nil)
	return c.metrics, c.findings
}

// tsFeatureCollector accumulates StructuralMetrics and Findings during a
// single pre-order walk of a TS/TSX tree.
type tsFeatureCollector struct {
	metrics          StructuralMetrics
	findings         []Finding
	mutatesInputSeen map[tsMutatesInputKey]bool
}

// mutatingTSMethodNames is the exact set of built-in Array/Map/Set method
// names whose call on a parameter-rooted receiver is treated as an
// in-place mutation of that parameter (Story 2). Arbitrary custom methods
// (e.g. `user.setName()`) are deliberately not in this set and so are
// never flagged.
var mutatingTSMethodNames = map[string]bool{
	"copyWithin": true,
	"fill":       true,
	"pop":        true,
	"push":       true,
	"reverse":    true,
	"shift":      true,
	"sort":       true,
	"splice":     true,
	"unshift":    true,
	"set":        true,
	"add":        true,
	"delete":     true,
	"clear":      true,
}

// tsParamScope is one function-like construct's Finding-name half
// ("<function_or_method_name>") plus the set of identifier bindings visible
// in that scope. A binding value of true means the identifier is a parameter
// eligible for mutates_input; false means the identifier is a local binding
// that shadows an outer parameter but is not itself reportable here.
type tsParamScope struct {
	ownerName string
	bindings  map[string]bool
}

// tsMutatesInputKey dedupes mutates_input findings by (owning function,
// parameter, mutation-expression location), mirroring the Go detector's
// dedup rule: repeated mutation of the same parameter through the same
// source location must not produce duplicate findings.
type tsMutatesInputKey struct {
	ownerName string
	paramName string
	startByte uint
	endByte   uint
}

// tsFunctionLikeKinds is the D2a "function-like-but-not-method" set:
// standalone/expression functions and arrows. Each one increments
// Functions and opens a function scope for the nesting rule (D2b).
// method_definition (class methods) is handled separately since it
// increments Methods instead and additionally participates in
// tight_coupling detection (D3).
var tsFunctionLikeKinds = map[string]bool{
	"function_declaration":           true,
	"function_expression":            true,
	"arrow_function":                 true,
	"generator_function_declaration": true,
	"generator_function":             true,
}

// walk visits n and its descendants in pre-order, incrementing metrics
// counters for the node kinds D2/D2a track and collecting tight_coupling
// findings (D3), all in the one traversal. blockDepth counts nested
// "statement_block" nodes seen so far within the current function-like
// body (0 outside any body); inFunc reports whether the walk is currently
// inside a function-like node (D2a), since nesting depth (D2b) is only
// measured inside those bodies. Entering any function-like node resets
// blockDepth to 0, so depth is measured per function body rather than
// cumulatively across nested functions -- exactly as Go resets on each
// function/method declaration.
//
// inCtorBody reports whether the walk is currently inside a constructor
// method's body: true only between entering a constructor method_definition
// and leaving its subtree. It is reset (to false, or to a fresh
// isConstructorMethod check) on every method_definition and on every
// non-arrow function-like node, since those introduce their own `this`
// binding -- so a nested class's constructor is scanned exactly once, by
// its own method_definition visit, not also by an enclosing constructor's
// scan, and a plain function nested inside a constructor does not
// misattribute its own `this.x = new Y()` to the enclosing constructor.
// Arrow functions do not rebind `this`, so descending into one preserves
// the enclosing inCtorBody value.
//
// scopes is the stack of enclosing function-like constructs' tsParamScope
// entries (mutates_input, Story 2/3), innermost last. Entering a
// method_definition or tsFunctionLikeKinds node pushes its own scope (name
// plus identifier-bound parameters) so that a mutation expression found
// anywhere in its subtree -- including inside a more deeply nested
// function/arrow that does not itself bind a same-named parameter --
// resolves to the correct owning construct by walking scopes innermost to
// outermost and matching the first one whose parameter set contains the
// mutated identifier (lexical shadowing, not just nearest-enclosing-node).
func (c *tsFeatureCollector) walk(n engine.Node, source []byte, blockDepth int, inFunc bool, inCtorBody bool, scopes []tsParamScope) {
	if n == nil {
		return
	}

	if n.Kind() == "catch_clause" {
		if names := tsCatchBindingNames(n, source); len(names) > 0 {
			scopes = appendTSLocalBindings(scopes, names)
		}
	}
	if names := tsControlFlowBindingNames(n, source); len(names) > 0 {
		scopes = appendTSLocalBindings(scopes, names)
	}

	switch {
	case n.Kind() == "if_statement":
		c.metrics.Ifs++
	case n.Kind() == "for_statement", n.Kind() == "for_in_statement":
		c.metrics.Fors++
	case n.Kind() == "switch_statement":
		c.metrics.ExprSwitches++
	case n.Kind() == "method_definition":
		c.metrics.Methods++
		inFunc = true
		blockDepth = 0
		inCtorBody = isConstructorMethod(n, source)
		scopes = append(scopes, newTSParamScope(n, source))
		scopes = appendTSLocalBindings(scopes, tsFunctionScopedBindingNames(n, source))
	case tsFunctionLikeKinds[n.Kind()]:
		c.metrics.Functions++
		inFunc = true
		blockDepth = 0
		if n.Kind() != "arrow_function" {
			inCtorBody = false
		}
		scopes = append(scopes, newTSParamScope(n, source))
		scopes = appendTSLocalBindings(scopes, tsFunctionScopedBindingNames(n, source))
	case n.Kind() == "statement_block":
		if inFunc {
			blockDepth++
			if blockDepth > c.metrics.MaxNestingDepth {
				c.metrics.MaxNestingDepth = blockDepth
			}
		}
	case inCtorBody && n.Kind() == "assignment_expression":
		c.checkTightCouplingAssignment(n, source)
	}

	if len(scopes) > 0 {
		switch n.Kind() {
		case "assignment_expression":
			c.checkMutatesInputAssignment(n, source, scopes)
		case "unary_expression":
			c.checkMutatesInputDelete(n, source, scopes)
		case "call_expression":
			c.checkMutatesInputCall(n, source, scopes)
		case "update_expression":
			c.checkMutatesInputUpdate(n, source, scopes)
		}
	}

	if n.Kind() == "statement_block" {
		scopes = appendTSLocalBindings(scopes, tsBlockScopedBindingNames(n, source))
		count := n.ChildCount()
		for i := 0; i < count; i++ {
			child := n.Child(i)
			c.walk(child, source, blockDepth, inFunc, inCtorBody, scopes)
			scopes = appendTSLocalBindings(scopes, tsLocalBindingNames(child, source))
			scopes = appendTSLocalBindings(scopes, tsReboundParameterNames(child, source))
			scopes = appendTSLocalBindings(scopes, tsVarBindingNames(child, source))
		}
		return
	}

	count := n.ChildCount()
	for i := 0; i < count; i++ {
		c.walk(n.Child(i), source, blockDepth, inFunc, inCtorBody, scopes)
	}
}

// newTSParamScope builds decl's tsParamScope: its Finding-name half (own
// "name" field's text, or "anonymous@<start_byte>" if it has none) and its
// identifier-bound parameter set (tsIdentifierParams).
func newTSParamScope(decl engine.Node, source []byte) tsParamScope {
	return tsParamScope{
		ownerName: tsFunctionOwnerName(decl, source),
		bindings:  tsIdentifierParams(decl, source),
	}
}

// tsFunctionOwnerName resolves decl's own Finding-name half: the source
// text of its syntactic "name" field (function_declaration,
// function_expression, generator_function[_declaration], and
// method_definition all expose one when named) or, when decl has no name
// field at all -- always true for arrow_function, and true for an
// anonymous function_expression -- "anonymous@<start_byte>". Per the
// issue spec this deliberately does not borrow a name from an enclosing
// variable_declarator (`const f = () => {}` still counts as anonymous):
// only decl's own syntactic name field counts.
func tsFunctionOwnerName(decl engine.Node, source []byte) string {
	if nameNode := decl.ChildByFieldName("name"); nameNode != nil {
		return nameNode.Utf8Text(source)
	}
	return fmt.Sprintf("anonymous@%d", decl.StartByte())
}

// tsIdentifierParams collects decl's plain-identifier-bound parameter
// names (D5). arrow_function has two mutually exclusive parameter shapes:
// a bare single identifier (`p => ...`, field "parameter") or a
// parenthesized formal_parameters list (field "parameters"); every other
// function-like kind and method_definition only ever have "parameters".
// Each formal_parameters child is a required_parameter or
// optional_parameter wrapping a "pattern" field, which is a plain
// identifier only for a non-destructured, non-rest binding; a "value"
// field present alongside it means the parameter has a default
// (`q = 1`), which -- per D5 -- is excluded just like the destructured and
// rest forms, not treated as identifier-bound.
func tsIdentifierParams(decl engine.Node, source []byte) map[string]bool {
	params := map[string]bool{}

	if decl.Kind() == "arrow_function" {
		if bare := decl.ChildByFieldName("parameter"); bare != nil {
			if bare.Kind() == "identifier" {
				params[bare.Utf8Text(source)] = true
			}
			return params
		}
	}

	formal := decl.ChildByFieldName("parameters")
	if formal == nil {
		return params
	}
	count := formal.ChildCount()
	for i := 0; i < count; i++ {
		p := formal.Child(i)
		if p.Kind() != "required_parameter" && p.Kind() != "optional_parameter" {
			continue
		}
		if p.ChildByFieldName("value") != nil {
			continue
		}
		pattern := p.ChildByFieldName("pattern")
		if pattern == nil || pattern.Kind() != "identifier" {
			continue
		}
		params[pattern.Utf8Text(source)] = true
	}
	return params
}

func tsFunctionScopedBindingNames(n engine.Node, source []byte) map[string]bool {
	names := map[string]bool{}
	var collect func(engine.Node)
	collect = func(node engine.Node) {
		if node == nil {
			return
		}
		if node != n && (tsFunctionLikeKinds[node.Kind()] || node.Kind() == "method_definition") {
			return
		}
		switch node.Kind() {
		case "function_declaration", "generator_function_declaration":
			if node != n {
				if name := node.ChildByFieldName("name"); name != nil {
					names[name.Utf8Text(source)] = true
				}
			}
			count := node.ChildCount()
			for i := 0; i < count; i++ {
				collect(node.Child(i))
			}
			return
		case "variable_declaration":
			count := node.ChildCount()
			for i := 0; i < count; i++ {
				collectTSVariableDeclaratorNames(node.Child(i), source, names)
			}
			return
		case "lexical_declaration":
			return
		}
		count := node.ChildCount()
		for i := 0; i < count; i++ {
			collect(node.Child(i))
		}
	}
	collect(n)
	return names
}

func tsBlockScopedBindingNames(n engine.Node, source []byte) map[string]bool {
	names := map[string]bool{}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		child := n.Child(i)
		switch child.Kind() {
		case "lexical_declaration":
			for j := 0; j < child.ChildCount(); j++ {
				collectTSVariableDeclaratorNames(child.Child(j), source, names)
			}
		case "function_declaration", "generator_function_declaration", "class_declaration":
			if name := child.ChildByFieldName("name"); name != nil {
				names[name.Utf8Text(source)] = true
			}
		}
	}
	return names
}

func appendTSLocalBindings(scopes []tsParamScope, names map[string]bool) []tsParamScope {
	if len(names) == 0 || len(scopes) == 0 {
		return scopes
	}
	next := make([]tsParamScope, len(scopes), len(scopes)+1)
	copy(next, scopes)
	next = append(next, tsParamScope{bindings: map[string]bool{}})
	for name := range names {
		next[len(next)-1].bindings[name] = false
	}
	return next
}

func tsLocalBindingNames(n engine.Node, source []byte) map[string]bool {
	if n == nil {
		return nil
	}
	switch n.Kind() {
	case "lexical_declaration", "variable_declaration":
		names := map[string]bool{}
		count := n.ChildCount()
		for i := 0; i < count; i++ {
			collectTSVariableDeclaratorNames(n.Child(i), source, names)
		}
		return names
	case "function_declaration", "generator_function_declaration":
		if name := n.ChildByFieldName("name"); name != nil {
			return map[string]bool{name.Utf8Text(source): true}
		}
		return nil
	case "class_declaration":
		if name := n.ChildByFieldName("name"); name != nil {
			return map[string]bool{name.Utf8Text(source): true}
		}
		return nil
	default:
		return nil
	}
}

func collectTSVariableDeclaratorNames(n engine.Node, source []byte, names map[string]bool) {
	if n == nil {
		return
	}
	if n.Kind() == "variable_declarator" {
		if name := n.ChildByFieldName("name"); name != nil {
			collectTSBindingPatternNames(name, source, names)
		}
		return
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		collectTSVariableDeclaratorNames(n.Child(i), source, names)
	}
}

func collectTSBindingPatternNames(n engine.Node, source []byte, names map[string]bool) {
	if n == nil {
		return
	}
	switch n.Kind() {
	case "identifier", "shorthand_property_identifier_pattern":
		names[n.Utf8Text(source)] = true
		return
	case "pair_pattern":
		if value := n.ChildByFieldName("value"); value != nil {
			collectTSBindingPatternNames(value, source, names)
		}
		return
	case "rest_pattern":
		if arg := n.ChildByFieldName("argument"); arg != nil {
			collectTSBindingPatternNames(arg, source, names)
		}
		return
	case "assignment_pattern":
		if left := n.ChildByFieldName("left"); left != nil {
			collectTSBindingPatternNames(left, source, names)
		}
		return
	default:
		count := n.ChildCount()
		for i := 0; i < count; i++ {
			collectTSBindingPatternNames(n.Child(i), source, names)
		}
	}
}

func tsControlFlowBindingNames(n engine.Node, source []byte) map[string]bool {
	if n == nil || (n.Kind() != "for_statement" && n.Kind() != "for_in_statement") {
		return nil
	}
	names := map[string]bool{}
	if left := n.ChildByFieldName("left"); left != nil {
		collectTSBindingPatternNames(left, source, names)
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		child := n.Child(i)
		switch child.Kind() {
		case "statement_block":
			return names
		case "lexical_declaration", "variable_declaration":
			for j := 0; j < child.ChildCount(); j++ {
				collectTSVariableDeclaratorNames(child.Child(j), source, names)
			}
		}
	}
	return names
}

func tsReboundParameterNames(n engine.Node, source []byte) map[string]bool {
	if n == nil {
		return nil
	}
	names := map[string]bool{}
	var collect func(engine.Node)
	collect = func(node engine.Node) {
		if node == nil {
			return
		}
		if tsFunctionLikeKinds[node.Kind()] || node.Kind() == "method_definition" {
			return
		}
		if node.Kind() == "assignment_expression" {
			if left := node.ChildByFieldName("left"); left != nil && left.Kind() == "identifier" {
				names[left.Utf8Text(source)] = true
			}
			return
		}
		count := node.ChildCount()
		for i := 0; i < count; i++ {
			collect(node.Child(i))
		}
	}
	collect(n)
	return names
}

func tsVarBindingNames(n engine.Node, source []byte) map[string]bool {
	if n == nil {
		return nil
	}
	names := map[string]bool{}
	var collect func(engine.Node)
	collect = func(node engine.Node) {
		if node == nil {
			return
		}
		if node.Kind() == "variable_declaration" {
			count := node.ChildCount()
			for i := 0; i < count; i++ {
				collectTSVariableDeclaratorNames(node.Child(i), source, names)
			}
			return
		}
		if tsFunctionLikeKinds[node.Kind()] || node.Kind() == "method_definition" {
			return
		}
		count := node.ChildCount()
		for i := 0; i < count; i++ {
			collect(node.Child(i))
		}
	}
	collect(n)
	return names
}

func tsCatchBindingNames(n engine.Node, source []byte) map[string]bool {
	if p := n.ChildByFieldName("parameter"); p != nil {
		names := map[string]bool{}
		collectTSBindingPatternNames(p, source, names)
		return names
	}
	return nil
}

// isConstructorMethod reports whether method is a constructor: a
// method_definition whose name field is a property_identifier with source
// text exactly "constructor".
func isConstructorMethod(method engine.Node, source []byte) bool {
	nameNode := method.ChildByFieldName("name")
	return nameNode != nil && nameNode.Kind() == "property_identifier" && nameNode.Utf8Text(source) == "constructor"
}

// checkTightCouplingAssignment emits a "tight_coupling" Finding (D3) if n
// (an assignment_expression within a constructor body) assigns a
// new_expression to a property of `this` -- `this.<prop> = new Y()` (a
// member_expression) or `this[<expr>] = new Y()` (a subscript_expression),
// each with an "object" field of kind "this". Assignments to a plain
// variable (`x = new Y()`) or to some other object's property
// (`other.x = new Y()`) are not tight coupling to the constructor's own
// instance and are excluded. variable_declarator initializers
// (`const c = new X()`) are never passed here (only assignment_expression
// nodes trigger this call).
func (c *tsFeatureCollector) checkTightCouplingAssignment(n engine.Node, source []byte) {
	left := n.ChildByFieldName("left")
	if left == nil || (left.Kind() != "member_expression" && left.Kind() != "subscript_expression") {
		return
	}
	if object := left.ChildByFieldName("object"); object == nil || object.Kind() != "this" {
		return
	}

	right := n.ChildByFieldName("right")
	if right == nil || right.Kind() != "new_expression" {
		return
	}
	name := ""
	if ctor := right.ChildByFieldName("constructor"); ctor != nil {
		name = ctor.Utf8Text(source)
	}
	c.findings = append(c.findings, Finding{
		Kind:     "tight_coupling",
		Name:     name,
		Location: locationFromNode(right),
	})
}

// checkMutatesInputAssignment emits a "mutates_input" Finding (Story 2) if
// n's left-hand side writes through a property or index rooted at some
// enclosing scope's identifier-bound parameter -- `p.x = ...`
// (member_expression) or `p[...] = ...` (subscript_expression), each with
// an "object" field resolving to a tracked parameter identifier. A plain
// identifier left-hand side (`p = other`) rebinds the local parameter
// variable rather than writing through it and is deliberately excluded.
// Evidence/Location are taken from the target (left-hand side) alone, not
// the whole assignment_expression: an assignment's right-hand side can be
// arbitrarily long (`p.x = someVeryLargeExpression()`), which would
// conflict with Evidence staying short, and would also diverge from the Go
// detector, whose evidence is likewise just the mutated selector/index
// target (e.g. cfg.Name), never including the assigned value.
func (c *tsFeatureCollector) checkMutatesInputAssignment(n engine.Node, source []byte, scopes []tsParamScope) {
	left := n.ChildByFieldName("left")
	base := tsMutationBase(left)
	if base == nil {
		return
	}
	c.recordMutatesInput(base, left, source, scopes)
}

// checkMutatesInputDelete emits a "mutates_input" Finding (Story 2) if n
// (a unary_expression) is a `delete` of a property or index rooted at some
// enclosing scope's identifier-bound parameter (`delete p.x`,
// `delete p['x']`). Unlike checkMutatesInputAssignment/checkMutatesInputCall,
// Evidence/Location are taken from n itself (the whole "delete ..."
// expression) rather than just the target: a delete unary_expression has no
// extra unbounded content beyond its "delete" keyword and target argument,
// so it is already short and bounded, and keeping the keyword makes the
// evidence self-explanatory as a deletion rather than a read.
func (c *tsFeatureCollector) checkMutatesInputDelete(n engine.Node, source []byte, scopes []tsParamScope) {
	operator := n.ChildByFieldName("operator")
	if operator == nil || operator.Utf8Text(source) != "delete" {
		return
	}
	base := tsMutationBase(n.ChildByFieldName("argument"))
	if base == nil {
		return
	}
	c.recordMutatesInput(base, n, source, scopes)
}

func (c *tsFeatureCollector) checkMutatesInputUpdate(n engine.Node, source []byte, scopes []tsParamScope) {
	target := n.ChildByFieldName("argument")
	base := tsMutationBase(target)
	if base == nil {
		return
	}
	c.recordMutatesInput(base, target, source, scopes)
}

// checkMutatesInputCall emits a "mutates_input" Finding (Story 2) if n (a
// call_expression) calls one of mutatingTSMethodNames on a receiver
// rooted at some enclosing scope's identifier-bound parameter, either
// directly (`p.push(x)`, `arr.sort()`, `m.set(k, v)`) or through a chain of
// nested member/subscript accesses (`p.items.push(1)`). Arbitrary custom
// method calls (`user.setName()`) are not in mutatingTSMethodNames and so
// never match. Evidence/Location are taken from fn (the receiver.method
// member_expression, e.g. "p.items.push"), not the whole call_expression:
// a call's arguments can be arbitrarily long or complex
// (`p.items.push(someVeryLargeExpression())`), which would conflict with
// Evidence staying short, and would also diverge from the Go detector's
// bounded, target-only evidence.
func (c *tsFeatureCollector) checkMutatesInputCall(n engine.Node, source []byte, scopes []tsParamScope) {
	fn := n.ChildByFieldName("function")
	if fn == nil {
		return
	}
	object, methodName := tsMutatingMethodReceiver(fn, source)
	if object == nil || !mutatingTSMethodNames[methodName] {
		return
	}
	base := tsResolveRootIdentifier(object)
	if base == nil {
		return
	}
	c.recordMutatesInput(base, fn, source, scopes)
}

func tsMutatingMethodReceiver(fn engine.Node, source []byte) (engine.Node, string) {
	switch fn.Kind() {
	case "member_expression":
		property := fn.ChildByFieldName("property")
		if property == nil {
			return nil, ""
		}
		return fn.ChildByFieldName("object"), property.Utf8Text(source)
	case "subscript_expression":
		index := fn.ChildByFieldName("index")
		methodName, ok := tsStringLiteralText(index, source)
		if !ok {
			return nil, ""
		}
		return fn.ChildByFieldName("object"), methodName
	default:
		return nil, ""
	}
}

func tsStringLiteralText(n engine.Node, source []byte) (string, bool) {
	if n == nil || n.Kind() != "string" {
		return "", false
	}
	value, err := strconv.Unquote(n.Utf8Text(source))
	if err != nil {
		return "", false
	}
	return value, true
}

// tsMutationBase resolves expr (a candidate mutation target/argument) down
// to the root identifier it is ultimately rooted at, when expr is a
// member_expression or subscript_expression -- either directly (`p.x`,
// `p[...]`) or through a chain of nested member_expression/
// subscript_expression "object" fields (`p.x.y`, `p.items[0].name`) -- or
// nil for any other shape, including a bare identifier (handled
// separately, since a bare identifier as an assignment's left-hand side is
// a rebind, not a write-through) and a chain that bottoms out in something
// other than a plain identifier (e.g. `f().x`), which is not resolved to a
// root.
func tsMutationBase(expr engine.Node) engine.Node {
	if expr == nil {
		return nil
	}
	if expr.Kind() != "member_expression" && expr.Kind() != "subscript_expression" {
		return nil
	}
	return tsResolveRootIdentifier(expr.ChildByFieldName("object"))
}

// tsResolveRootIdentifier walks a chain of nested member_expression/
// subscript_expression "object" fields, starting at expr, until it reaches
// a plain identifier -- the root -- or determines there is no such root
// (e.g. the chain bottoms out in a call_expression like `f().x`), in which
// case it returns nil. Used by both tsMutationBase (assignment/delete
// targets) and checkMutatesInputCall (method-call receivers) so nested
// mutation targets/receivers rooted at a tracked parameter (`p.x.y = 1`,
// `p.items.push(1)`) are resolved the same way.
func tsResolveRootIdentifier(expr engine.Node) engine.Node {
	for expr != nil {
		switch expr.Kind() {
		case "identifier":
			return expr
		case "member_expression", "subscript_expression":
			expr = expr.ChildByFieldName("object")
		case "parenthesized_expression", "non_null_expression":
			expr = tsWrappedExpressionInner(expr)
		default:
			return nil
		}
	}
	return nil
}

func tsWrappedExpressionInner(expr engine.Node) engine.Node {
	for _, field := range []string{"expression", "operand", "argument"} {
		if child := expr.ChildByFieldName(field); child != nil {
			return child
		}
	}
	count := expr.ChildCount()
	for i := 0; i < count; i++ {
		child := expr.Child(i)
		switch child.Kind() {
		case "(", ")", "!":
			continue
		default:
			return child
		}
	}
	return nil
}

// recordMutatesInput resolves base's identifier name against scopes
// (innermost to outermost, so a nested function's own same-named parameter
// shadows an outer one -- D6) and, if it is a tracked identifier-bound
// parameter of some scope, records a deduplicated "mutates_input" Finding
// attributing the mutation at evidence's own source span to that scope's
// owner name.
func (c *tsFeatureCollector) recordMutatesInput(base engine.Node, evidence engine.Node, source []byte, scopes []tsParamScope) {
	name := base.Utf8Text(source)
	var owner string
	found := false
	for i := len(scopes) - 1; i >= 0; i-- {
		isParam, ok := scopes[i].bindings[name]
		if !ok {
			continue
		}
		if !isParam {
			return
		}
		owner = scopes[i].ownerName
		found = true
		break
	}
	if !found {
		return
	}

	loc := locationFromNode(evidence)
	key := tsMutatesInputKey{ownerName: owner, paramName: name, startByte: loc.StartByte, endByte: loc.EndByte}
	if c.mutatesInputSeen == nil {
		c.mutatesInputSeen = map[tsMutatesInputKey]bool{}
	}
	if c.mutatesInputSeen[key] {
		return
	}
	c.mutatesInputSeen[key] = true

	c.findings = append(c.findings, newMutatesInputFinding(owner, name, evidence, source))
}
