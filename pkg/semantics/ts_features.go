package semantics

import (
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

// checkMutatesInputForNode runs the mutates_input detector (Story 2)
// matching n's own kind, when scopes has at least one enclosing
// function-like/method scope to attribute a mutation to.

// walkScopedChildBlock walks n's children in declaration order, threading a
// scopes stack extended first by n's own hoisted binding names (scopeNames
// -- tsBlockScopedBindingNames for statement_block/switch_case/
// switch_default, tsSwitchBodyBindingNames for switch_body, the only way
// those three node kinds differ here) and then, after each child, that
// child's own local/rebound/var binding after-effects -- so a later sibling
// sees bindings a plain pre-order walk would not have introduced yet.

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

// tsIdentifierParams collects decl's plain-identifier-bound parameter
// names (D5). arrow_function has two mutually exclusive parameter shapes:
// a bare single identifier (`p => ...`, field "parameter") or a
// parenthesized formal_parameters list (field "parameters"); every other
// function-like kind and method_definition only ever have "parameters".
// Each formal_parameters child is filtered per-parameter by
// tsFormalParameterIdentifierName, whose doc comment is the source of
// truth for what counts as identifier-bound.

// tsFormalParameterIdentifierName reports p's bound identifier name with ok
// == true only when p is a required_parameter or optional_parameter with no
// default "value" field (a default like `q = 1` is excluded, per D5, same
// as a destructured or rest parameter) whose "pattern" field is itself a
// plain, non-destructured identifier.

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

// collectTSFunctionDeclarationNames handles the
// function_declaration/generator_function_declaration case of
// tsCollectFunctionScopedNodeNames: node is always root here --
// collectTSFunctionScopedBindingNames' function-like boundary check already
// stops at any nested function_declaration, so the node != root
// name-collection guard below is defensive and never fires (behavior
// preserved verbatim from the pre-refactor collect closure) -- then
// recursion into node's own children.
type nameSet map[string]bool

// collectTSFunctionScopedVarDeclarationNames handles the
// variable_declaration case of tsCollectFunctionScopedNodeNames: node's own
// `var`-bound declarator names, excluding any already in params.

// isConstructorMethod reports whether method is a constructor: a
// method_definition whose name field is a property_identifier with source
// text exactly "constructor".
func isConstructorMethod(method engine.Node, source []byte) bool {
	nameNode := method.ChildByFieldName("name")
	return nameNode != nil && nameNode.Kind() == "property_identifier" && nameNode.Utf8Text(source) == "constructor"
}
