package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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
