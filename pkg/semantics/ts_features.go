package semantics

import (
	"fmt"

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
// ("<function_or_method_name>") plus the set of its identifier-bound
// parameter names. Only plain-identifier parameter bindings are ever
// recorded here (D5): object/array/rest/assignment-pattern parameters are
// skipped entirely when a scope is built, so a mutation expression rooted
// at one of them never resolves to any scope.
type tsParamScope struct {
	ownerName string
	params    map[string]bool
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
	case tsFunctionLikeKinds[n.Kind()]:
		c.metrics.Functions++
		inFunc = true
		blockDepth = 0
		if n.Kind() != "arrow_function" {
			inCtorBody = false
		}
		scopes = append(scopes, newTSParamScope(n, source))
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
		}
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
		params:    tsIdentifierParams(decl, source),
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
	if fn == nil || fn.Kind() != "member_expression" {
		return
	}
	property := fn.ChildByFieldName("property")
	if property == nil || !mutatingTSMethodNames[property.Utf8Text(source)] {
		return
	}
	object := fn.ChildByFieldName("object")
	base := tsResolveRootIdentifier(object)
	if base == nil {
		return
	}
	c.recordMutatesInput(base, fn, source, scopes)
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
		default:
			return nil
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
		if scopes[i].params[name] {
			owner = scopes[i].ownerName
			found = true
			break
		}
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

	c.findings = append(c.findings, Finding{
		Kind:           "mutates_input",
		Name:           owner + ":" + name,
		Location:       loc,
		Confidence:     "medium",
		Evidence:       evidence.Utf8Text(source),
		Recommendation: "Return a copy instead of mutating the caller's value, or document/rename this function to make the in-place mutation explicit.",
		SuggestedSkill: "refactor-hidden-mutation",
	})
}
