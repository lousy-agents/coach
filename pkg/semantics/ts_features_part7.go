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
func (s nameSet) collectBindingPatternNamesExcept(n engine.Node, source []byte, except map[string]bool) {
	all := nameSet{}
	all.collectBindingPatternNames(n, source)
	for name := range all {
		if except[name] {
			continue
		}
		s[name] = true
	}
}
