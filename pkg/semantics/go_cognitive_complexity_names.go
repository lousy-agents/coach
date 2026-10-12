package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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
