package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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
		nameSet(names).collectFunctionDeclarationNames(root, node, source, params)
		return true
	case "variable_declaration":
		nameSet(names).collectFunctionScopedVarDeclarationNames(node, source, params)
		return true
	case "lexical_declaration":
		return true
	default:
		return false
	}
}
func tsCatchBindingNames(n engine.Node, source []byte) map[string]bool {
	if p := n.ChildByFieldName("parameter"); p != nil {
		names := nameSet{}
		names.collectBindingPatternNames(p, source)
		return names
	}
	return nil
}
