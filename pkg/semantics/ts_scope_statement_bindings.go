package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func tsLocalBindingNames(n engine.Node, source []byte, currentParams map[string]bool) map[string]bool {
	if n == nil {
		return nil
	}
	switch n.Kind() {
	case "lexical_declaration":
		names := map[string]bool{}
		count := n.ChildCount()
		for i := 0; i < count; i++ {
			collectTSVariableDeclaratorNames(n.Child(i), source, names)
		}
		return names
	case "variable_declaration":
		names := map[string]bool{}
		collectTSVariableDeclaratorNamesAfterStatement(n, source, currentParams, names)
		return names
	case "function_declaration", "generator_function_declaration":
		if name := n.ChildByFieldName("name"); name != nil {
			return map[string]bool{name.Utf8Text(source): true}
		}
		return nil
	case "class_declaration":
		if name := n.ChildByFieldName("name"); name != nil {
			return map[string]bool{name.Utf8Text(source): true}
		}
		return nil
	default:
		return nil
	}
}

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
