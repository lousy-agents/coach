package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// mutableParamTypes maps each declared parameter identifier to its
// paramMutKind, derived from whether its syntactic type is a pointer_type
// (paramMutPointer) or a map_type/slice_type (paramMutCollection). Go
// allows a single parameter_declaration to bind multiple names to one
// shared type (func f(a, b *T)), so every identifier child of each
// parameter_declaration is collected, not just the first.
func mutableParamTypes(params engine.Node, source []byte) map[string]paramMutKind {
	result := map[string]paramMutKind{}
	count := params.ChildCount()
	for i := 0; i < count; i++ {
		decl := params.Child(i)
		if decl.Kind() != "parameter_declaration" {
			continue
		}
		typeNode := decl.ChildByFieldName("type")
		kind := paramNotMutable
		if typeNode != nil {
			switch typeNode.Kind() {
			case "pointer_type":
				kind = paramMutPointer
			case "map_type", "slice_type":
				kind = paramMutCollection
			}
		}

		declCount := decl.ChildCount()
		for j := 0; j < declCount; j++ {
			nameChild := decl.Child(j)
			if nameChild.Kind() != "identifier" {
				continue
			}
			result[nameChild.Utf8Text(source)] = kind
		}
	}
	return result
}
func identifiersInVarDeclaration(n engine.Node, source []byte) map[string]bool {
	names := map[string]bool{}
	var collect func(engine.Node)
	collect = func(node engine.Node) {
		if node == nil {
			return
		}
		if node.Kind() == "var_spec" {
			if name := node.ChildByFieldName("name"); name != nil {
				identSet(names).collect(name, source)
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
