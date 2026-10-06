package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// paramMutKind classifies a declared parameter's syntactic type for
// mutates_input purposes. Selector/dereference writes (cfg.Name = x,
// (*cfg).Name = x) are only caller-visible through a pointer, and index
// writes (values[k] = x, items[i] = x) are only caller-visible through a
// map or slice -- collapsing these into a single bool would let a
// selector write on a map/slice parameter, or an index write on a
// (non-array) pointer parameter, be misreported as mutates_input even
// though neither is a caller-visible write for that parameter's actual
// type.
type paramMutKind uint8

const (
	// paramNotMutable is also the zero value, so a parameter absent from
	// a paramMutKind map (or explicitly recorded as such) is never
	// mistaken for a pointer/collection parameter.
	paramNotMutable paramMutKind = iota
	paramMutPointer
	paramMutCollection
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
