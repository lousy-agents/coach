package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func tsControlFlowBindingNames(n engine.Node, source []byte) map[string]bool {
	if n == nil || (n.Kind() != "for_statement" && n.Kind() != "for_in_statement") {
		return nil
	}
	names := map[string]bool{}
	if left := n.ChildByFieldName("left"); left != nil {
		nameSet(names).collectBindingPatternNames(left, source)
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
			nameSet(names).collectBindingPatternNamesExcept(name, source, currentParams)
			return
		}
		nameSet(names).collectBindingPatternNames(name, source)
		return
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		collectTSVariableDeclaratorNamesAfterStatement(n.Child(i), source, currentParams, names)
	}
}
