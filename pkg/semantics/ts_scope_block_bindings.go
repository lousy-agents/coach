package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func tsBlockScopedBindingNames(n engine.Node, source []byte) map[string]bool {
	names := map[string]bool{}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		child := n.Child(i)
		switch child.Kind() {
		case "lexical_declaration":
			for j := 0; j < child.ChildCount(); j++ {
				collectTSVariableDeclaratorNames(child.Child(j), source, names)
			}
		case "function_declaration", "generator_function_declaration", "class_declaration":
			if name := child.ChildByFieldName("name"); name != nil {
				names[name.Utf8Text(source)] = true
			}
		}
	}
	return names
}

func tsSwitchBodyBindingNames(n engine.Node, source []byte) map[string]bool {
	names := map[string]bool{}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		child := n.Child(i)
		if child.Kind() != "switch_case" && child.Kind() != "switch_default" {
			continue
		}
		for name := range tsBlockScopedBindingNames(child, source) {
			names[name] = true
		}
	}
	return names
}
