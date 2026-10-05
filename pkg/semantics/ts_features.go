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

// isConstructorMethod reports whether method is a constructor: a
// method_definition whose name field is a property_identifier with source
// text exactly "constructor".
func isConstructorMethod(method engine.Node, source []byte) bool {
	nameNode := method.ChildByFieldName("name")
	return nameNode != nil && nameNode.Kind() == "property_identifier" && nameNode.Utf8Text(source) == "constructor"
}
