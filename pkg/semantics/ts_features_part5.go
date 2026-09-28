package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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
func collectTSVariableDeclaratorNames(n engine.Node, source []byte, names map[string]bool) {
	if n == nil {
		return
	}
	if n.Kind() == "variable_declarator" {
		if name := n.ChildByFieldName("name"); name != nil {
			nameSet(names).collectBindingPatternNames(name, source)
		}
		return
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		collectTSVariableDeclaratorNames(n.Child(i), source, names)
	}
}
