package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func tsReboundParameterNames(n engine.Node, source []byte) map[string]bool {
	if n == nil {
		return nil
	}
	collected := &reboundNames{source: source, names: map[string]bool{}}
	collected.collect(n)
	return collected.names
}

type reboundNames struct {
	source []byte
	names  map[string]bool
}

func (r *reboundNames) collect(node engine.Node) {
	if node == nil {
		return
	}
	if tsFunctionLikeKinds[node.Kind()] || node.Kind() == "method_definition" {
		return
	}
	if node.Kind() == "assignment_expression" || node.Kind() == "augmented_assignment_expression" {
		if left := node.ChildByFieldName("left"); left != nil {
			nameSet(r.names).collectReboundTargetNames(left, r.source)
		}
		return
	}
	count := node.ChildCount()
	for i := 0; i < count; i++ {
		r.collect(node.Child(i))
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
