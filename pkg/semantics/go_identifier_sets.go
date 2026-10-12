package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

type identSet map[string]bool

func (s identSet) collect(n engine.Node, source []byte) {
	if n == nil {
		return
	}
	if n.Kind() == "identifier" {
		s[n.Utf8Text(source)] = true
		return
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		s.collect(n.Child(i), source)
	}
}

func (s identSet) collectParams(n engine.Node, source []byte) {
	if n == nil {
		return
	}
	switch n.Kind() {
	case "parameter_declaration", "variadic_parameter_declaration":
		count := n.ChildCount()
		for i := 0; i < count; i++ {
			child := n.Child(i)
			if child.Kind() == "identifier" {
				s[child.Utf8Text(source)] = true
			}
		}
		return
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		s.collectParams(n.Child(i), source)
	}
}

func identifiersInNodeField(n engine.Node, field string, source []byte) map[string]bool {
	child := n.ChildByFieldName(field)
	if child == nil {
		return nil
	}
	names := identSet{}
	names.collect(child, source)
	return names
}

func parameterNames(params engine.Node, source []byte) map[string]bool {
	names := map[string]bool{}
	if params == nil {
		return names
	}
	count := params.ChildCount()
	for i := 0; i < count; i++ {
		identSet(names).collectParams(params.Child(i), source)
	}
	return names
}
