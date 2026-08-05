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
	toctouActSeen    map[tsLocationKey]bool
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

	blockDepth, inFunc, inCtorBody, scopes = c.walkEnterNode(n, source, blockDepth, inFunc, inCtorBody, scopes)
	c.checkMutatesInputForNode(n, source, scopes)

	switch n.Kind() {
	case "statement_block":
		c.walkScopedChildBlock(n, source, blockDepth, inFunc, inCtorBody, scopes, tsBlockScopedBindingNames)
		return
	case "switch_body":
		c.walkScopedChildBlock(n, source, blockDepth, inFunc, inCtorBody, scopes, tsSwitchBodyBindingNames)
		return
	case "switch_case", "switch_default":
		c.walkScopedChildBlock(n, source, blockDepth, inFunc, inCtorBody, scopes, tsBlockScopedBindingNames)
		return
	}

	count := n.ChildCount()
	for i := 0; i < count; i++ {
		c.walk(n.Child(i), source, blockDepth, inFunc, inCtorBody, scopes)
	}
}

// walkEnterNode applies walk's per-node-kind metrics increments, the
// TOCTOU/tight-coupling finding checks that fire on entering (not
// descending into) n, and the resulting updates to the per-descent state
// (blockDepth, inFunc, inCtorBody, scopes) that walk threads through the
// rest of n's subtree. See walk's own doc comment for the exact
// reset/nesting contract each state field encodes.
func (c *tsFeatureCollector) walkEnterNode(n engine.Node, source []byte, blockDepth int, inFunc bool, inCtorBody bool, scopes []tsParamScope) (int, bool, bool, []tsParamScope) {
	switch {
	case n.Kind() == "if_statement":
		c.metrics.Ifs++
		c.checkTOCTOUCheckThenAct(n, source)
	case n.Kind() == "while_statement":
		c.checkTOCTOUCheckThenAct(n, source)
	case n.Kind() == "for_statement", n.Kind() == "for_in_statement":
		c.metrics.Fors++
	case n.Kind() == "switch_statement":
		c.metrics.ExprSwitches++
	case n.Kind() == "method_definition":
		c.metrics.Methods++
		inFunc = true
		blockDepth = 0
		inCtorBody = isConstructorMethod(n, source)
		scope := newTSParamScope(n, source)
		scopes = append(scopes, scope)
		scopes = appendTSLocalBindings(scopes, tsFunctionScopedBindingNames(n, source, scope.bindings))
	case tsFunctionLikeKinds[n.Kind()]:
		c.metrics.Functions++
		inFunc = true
		blockDepth = 0
		if n.Kind() != "arrow_function" {
			inCtorBody = false
		}
		scope := newTSParamScope(n, source)
		scopes = append(scopes, scope)
		scopes = appendTSLocalBindings(scopes, tsFunctionScopedBindingNames(n, source, scope.bindings))
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
	return blockDepth, inFunc, inCtorBody, scopes
}

// checkMutatesInputForNode runs the mutates_input detector (Story 2)
// matching n's own kind, when scopes has at least one enclosing
// function-like/method scope to attribute a mutation to.
func (c *tsFeatureCollector) checkMutatesInputForNode(n engine.Node, source []byte, scopes []tsParamScope) {
	if len(scopes) == 0 {
		return
	}
	switch n.Kind() {
	case "assignment_expression", "augmented_assignment_expression":
		c.checkMutatesInputAssignment(n, source, scopes)
	case "unary_expression":
		c.checkMutatesInputDelete(n, source, scopes)
	case "call_expression":
		c.checkMutatesInputCall(n, source, scopes)
	case "update_expression":
		c.checkMutatesInputUpdate(n, source, scopes)
	}
}

// walkScopedChildBlock walks n's children in declaration order, threading a
// scopes stack extended first by n's own hoisted binding names (scopeNames
// -- tsBlockScopedBindingNames for statement_block/switch_case/
// switch_default, tsSwitchBodyBindingNames for switch_body, the only way
// those three node kinds differ here) and then, after each child, that
// child's own local/rebound/var binding after-effects -- so a later sibling
// sees bindings a plain pre-order walk would not have introduced yet.
func (c *tsFeatureCollector) walkScopedChildBlock(n engine.Node, source []byte, blockDepth int, inFunc bool, inCtorBody bool, scopes []tsParamScope, scopeNames func(engine.Node, []byte) map[string]bool) {
	scopes = appendTSLocalBindings(scopes, scopeNames(n, source))
	currentParams := tsCurrentFunctionParamNames(scopes)
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		child := n.Child(i)
		c.walk(child, source, blockDepth, inFunc, inCtorBody, scopes)
		scopes = appendTSLocalBindings(scopes, tsLocalBindingNames(child, source, currentParams))
		scopes = appendTSLocalBindings(scopes, tsReboundParameterNames(child, source))
		scopes = appendTSLocalBindings(scopes, tsVarBindingNames(child, source, currentParams))
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
// Each formal_parameters child is filtered per-parameter by
// tsFormalParameterIdentifierName, whose doc comment is the source of
// truth for what counts as identifier-bound.
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
		if name, ok := tsFormalParameterIdentifierName(formal.Child(i), source); ok {
			params[name] = true
		}
	}
	return params
}

// tsFormalParameterIdentifierName reports p's bound identifier name with ok
// == true only when p is a required_parameter or optional_parameter with no
// default "value" field (a default like `q = 1` is excluded, per D5, same
// as a destructured or rest parameter) whose "pattern" field is itself a
// plain, non-destructured identifier.
func tsFormalParameterIdentifierName(p engine.Node, source []byte) (string, bool) {
	if p.Kind() != "required_parameter" && p.Kind() != "optional_parameter" {
		return "", false
	}
	if p.ChildByFieldName("value") != nil {
		return "", false
	}
	pattern := p.ChildByFieldName("pattern")
	if pattern == nil || pattern.Kind() != "identifier" {
		return "", false
	}
	return pattern.Utf8Text(source), true
}

func tsFunctionScopedBindingNames(n engine.Node, source []byte, params map[string]bool) map[string]bool {
	names := map[string]bool{}
	collectTSFunctionScopedBindingNames(n, n, source, params, names)
	return names
}

// collectTSFunctionScopedBindingNames recurses node's subtree relative to
// root (the enclosing function-like/method_definition n started from in
// tsFunctionScopedBindingNames), stopping without descending at any nested
// function-like or method_definition boundary other than root itself, and
// otherwise delegating node's own contribution to
// tsCollectFunctionScopedNodeNames.
func collectTSFunctionScopedBindingNames(root, node engine.Node, source []byte, params, names map[string]bool) {
	if node == nil {
		return
	}
	if node != root && (tsFunctionLikeKinds[node.Kind()] || node.Kind() == "method_definition") {
		return
	}
	if tsCollectFunctionScopedNodeNames(root, node, source, params, names) {
		return
	}
	count := node.ChildCount()
	for i := 0; i < count; i++ {
		collectTSFunctionScopedBindingNames(root, node.Child(i), source, params, names)
	}
}

// tsCollectFunctionScopedNodeNames handles node's own hoisted-binding
// contribution when node is a function_declaration/
// generator_function_declaration, variable_declaration, or
// lexical_declaration, reporting handled == true so
// collectTSFunctionScopedBindingNames does not also apply its own generic
// child recursion for these three kinds (each either recurses itself, or --
// lexical_declaration, since let/const are block-scoped, not hoisted --
// must not recurse into its subtree at all).
func tsCollectFunctionScopedNodeNames(root, node engine.Node, source []byte, params, names map[string]bool) bool {
	switch node.Kind() {
	case "function_declaration", "generator_function_declaration":
		collectTSFunctionDeclarationNames(root, node, source, params, names)
		return true
	case "variable_declaration":
		collectTSFunctionScopedVarDeclarationNames(node, source, params, names)
		return true
	case "lexical_declaration":
		return true
	default:
		return false
	}
}

// collectTSFunctionDeclarationNames handles the
// function_declaration/generator_function_declaration case of
// tsCollectFunctionScopedNodeNames: node is always root here --
// collectTSFunctionScopedBindingNames' function-like boundary check already
// stops at any nested function_declaration, so the node != root
// name-collection guard below is defensive and never fires (behavior
// preserved verbatim from the pre-refactor collect closure) -- then
// recursion into node's own children.
func collectTSFunctionDeclarationNames(root, node engine.Node, source []byte, params, names map[string]bool) {
	if node != root {
		if name := node.ChildByFieldName("name"); name != nil {
			names[name.Utf8Text(source)] = true
		}
	}
	count := node.ChildCount()
	for i := 0; i < count; i++ {
		collectTSFunctionScopedBindingNames(root, node.Child(i), source, params, names)
	}
}

// collectTSFunctionScopedVarDeclarationNames handles the
// variable_declaration case of tsCollectFunctionScopedNodeNames: node's own
// `var`-bound declarator names, excluding any already in params.
func collectTSFunctionScopedVarDeclarationNames(node engine.Node, source []byte, params, names map[string]bool) {
	varNames := map[string]bool{}
	count := node.ChildCount()
	for i := 0; i < count; i++ {
		collectTSVariableDeclaratorNames(node.Child(i), source, varNames)
	}
	for name := range varNames {
		if !params[name] {
			names[name] = true
		}
	}
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

func tsSwitchBodyBindingNames(n engine.Node, source []byte) map[string]bool {
	names := map[string]bool{}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		child := n.Child(i)
		if child.Kind() != "switch_case" && child.Kind() != "switch_default" {
			continue
		}
		for name := range tsBlockScopedBindingNames(child, source) {
			names[name] = true
		}
	}
	return names
}

func appendTSLocalBindings(scopes []tsParamScope, names map[string]bool) []tsParamScope {
	if len(names) == 0 || len(scopes) == 0 {
		return scopes
	}
	bindings := make(map[string]bool, len(names))
	for name := range names {
		bindings[name] = false
	}
	return append(scopes, tsParamScope{bindings: bindings})
}

func tsCurrentFunctionParamNames(scopes []tsParamScope) map[string]bool {
	for i := len(scopes) - 1; i >= 0; i-- {
		if scopes[i].ownerName == "" {
			continue
		}
		names := map[string]bool{}
		for name, isParam := range scopes[i].bindings {
			if isParam {
				names[name] = true
			}
		}
		return names
	}
	return nil
}

func tsLocalBindingNames(n engine.Node, source []byte, currentParams map[string]bool) map[string]bool {
	if n == nil {
		return nil
	}
	switch n.Kind() {
	case "lexical_declaration":
		names := map[string]bool{}
		count := n.ChildCount()
		for i := 0; i < count; i++ {
			collectTSVariableDeclaratorNames(n.Child(i), source, names)
		}
		return names
	case "variable_declaration":
		names := map[string]bool{}
		collectTSVariableDeclaratorNamesAfterStatement(n, source, currentParams, names)
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

func collectTSVariableDeclaratorNamesAfterStatement(n engine.Node, source []byte, currentParams map[string]bool, names map[string]bool) {
	if n == nil {
		return
	}
	if n.Kind() == "variable_declarator" {
		name := n.ChildByFieldName("name")
		if name == nil {
			return
		}
		if n.ChildByFieldName("value") == nil {
			collectTSBindingPatternNamesExcept(name, source, currentParams, names)
			return
		}
		collectTSBindingPatternNames(name, source, names)
		return
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		collectTSVariableDeclaratorNamesAfterStatement(n.Child(i), source, currentParams, names)
	}
}

func collectTSBindingPatternNamesExcept(n engine.Node, source []byte, except map[string]bool, names map[string]bool) {
	all := map[string]bool{}
	collectTSBindingPatternNames(n, source, all)
	for name := range all {
		if except[name] {
			continue
		}
		names[name] = true
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
		if node.Kind() == "assignment_expression" || node.Kind() == "augmented_assignment_expression" {
			if left := node.ChildByFieldName("left"); left != nil {
				collectTSReboundTargetNames(left, source, names)
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

func collectTSReboundTargetNames(n engine.Node, source []byte, names map[string]bool) {
	if n == nil {
		return
	}
	switch n.Kind() {
	case "identifier", "shorthand_property_identifier_pattern":
		names[n.Utf8Text(source)] = true
	case "pair_pattern":
		collectTSReboundTargetNames(n.ChildByFieldName("value"), source, names)
	case "assignment_pattern":
		collectTSReboundTargetNames(n.ChildByFieldName("left"), source, names)
	case "rest_pattern":
		collectTSReboundTargetNames(n.ChildByFieldName("argument"), source, names)
	case "object_pattern", "array_pattern", "parenthesized_expression":
		count := n.ChildCount()
		for i := 0; i < count; i++ {
			collectTSReboundTargetNames(n.Child(i), source, names)
		}
	}
}

func tsVarBindingNames(n engine.Node, source []byte, currentParams map[string]bool) map[string]bool {
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
				collectTSVariableDeclaratorNamesAfterStatement(node.Child(i), source, currentParams, names)
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
