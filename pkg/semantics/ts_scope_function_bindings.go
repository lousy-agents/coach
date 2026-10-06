package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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

// collectFunctionDeclarationNames handles the
// function_declaration/generator_function_declaration case of
// tsCollectFunctionScopedNodeNames: node is always root here --
// collectTSFunctionScopedBindingNames' function-like boundary check already
// stops at any nested function_declaration, so the node != root
// name-collection guard below is defensive and never fires (behavior
// preserved verbatim from the pre-refactor collect closure) -- then
// recursion into node's own children.
func (s nameSet) collectFunctionDeclarationNames(root, node engine.Node, source []byte, params map[string]bool) {
	if node != root {
		if name := node.ChildByFieldName("name"); name != nil {
			s[name.Utf8Text(source)] = true
		}
	}
	count := node.ChildCount()
	for i := 0; i < count; i++ {
		collectTSFunctionScopedBindingNames(root, node.Child(i), source, params, s)
	}
}

// collectFunctionScopedVarDeclarationNames handles the
// variable_declaration case of tsCollectFunctionScopedNodeNames: node's own
// `var`-bound declarator names, excluding any already in params.
func (s nameSet) collectFunctionScopedVarDeclarationNames(node engine.Node, source []byte, params map[string]bool) {
	varNames := map[string]bool{}
	count := node.ChildCount()
	for i := 0; i < count; i++ {
		collectTSVariableDeclaratorNames(node.Child(i), source, varNames)
	}
	for name := range varNames {
		if !params[name] {
			s[name] = true
		}
	}
}
