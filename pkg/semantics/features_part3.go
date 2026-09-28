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
	kinds := &paramKindSet{result: map[string]paramMutKind{}}
	count := params.ChildCount()
	for i := 0; i < count; i++ {
		kinds.addDecl(params.Child(i), source)
	}
	return kinds.result
}

type paramKindSet struct {
	result map[string]paramMutKind
}

func (p *paramKindSet) addDecl(decl engine.Node, source []byte) {
	if decl.Kind() != "parameter_declaration" {
		return
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
		p.result[nameChild.Utf8Text(source)] = kind
	}
}
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
