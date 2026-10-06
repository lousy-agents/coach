package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func identifiersInVarDeclaration(n engine.Node, source []byte) map[string]bool {
	collected := &varDeclNames{source: source, names: map[string]bool{}}
	collected.collect(n)
	return collected.names
}

type varDeclNames struct {
	source []byte
	names  map[string]bool
}

func (v *varDeclNames) collect(node engine.Node) {
	if node == nil {
		return
	}
	if node.Kind() == "var_spec" {
		if name := node.ChildByFieldName("name"); name != nil {
			identSet(v.names).collect(name, v.source)
		}
		return
	}
	count := node.ChildCount()
	for i := 0; i < count; i++ {
		v.collect(node.Child(i))
	}
}

func identifiersDeclaredInSubtree(n engine.Node, source []byte) map[string]bool {
	names := map[string]bool{}
	var collect func(engine.Node)
	collect = func(node engine.Node) {
		if node == nil {
			return
		}
		switch node.Kind() {
		case "short_var_declaration":
			for name := range identifiersInNodeField(node, "left", source) {
				names[name] = true
			}
			return
		case "var_declaration":
			for name := range identifiersInVarDeclaration(node, source) {
				names[name] = true
			}
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
