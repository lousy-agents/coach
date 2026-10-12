package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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
