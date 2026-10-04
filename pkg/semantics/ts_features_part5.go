package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func tsVarBindingNames(n engine.Node, source []byte, currentParams map[string]bool) map[string]bool {
	if n == nil {
		return nil
	}
	collected := &varBindingNames{source: source, currentParams: currentParams, names: map[string]bool{}}
	collected.collect(n)
	return collected.names
}

type varBindingNames struct {
	source        []byte
	currentParams map[string]bool
	names         map[string]bool
}

func (v *varBindingNames) collect(node engine.Node) {
	if node == nil {
		return
	}
	if node.Kind() == "variable_declaration" {
		count := node.ChildCount()
		for i := 0; i < count; i++ {
			collectTSVariableDeclaratorNamesAfterStatement(node.Child(i), v.source, v.currentParams, v.names)
		}
		return
	}
	if tsFunctionLikeKinds[node.Kind()] || node.Kind() == "method_definition" {
		return
	}
	count := node.ChildCount()
	for i := 0; i < count; i++ {
		v.collect(node.Child(i))
	}
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
