package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func tsReboundParameterNames(n engine.Node, source []byte) map[string]bool {
	if n == nil {
		return nil
	}
	collected := &reboundNames{source: source, names: map[string]bool{}}
	collected.collect(n)
	return collected.names
}

type reboundNames struct {
	source []byte
	names  map[string]bool
}

func (r *reboundNames) collect(node engine.Node) {
	if node == nil {
		return
	}
	if tsFunctionLikeKinds[node.Kind()] || node.Kind() == "method_definition" {
		return
	}
	if node.Kind() == "assignment_expression" || node.Kind() == "augmented_assignment_expression" {
		if left := node.ChildByFieldName("left"); left != nil {
			nameSet(r.names).collectReboundTargetNames(left, r.source)
		}
		return
	}
	count := node.ChildCount()
	for i := 0; i < count; i++ {
		r.collect(node.Child(i))
	}
}

func (s nameSet) collectReboundTargetNames(n engine.Node, source []byte) {
	if n == nil {
		return
	}
	switch n.Kind() {
	case "identifier", "shorthand_property_identifier_pattern":
		s[n.Utf8Text(source)] = true
	case "pair_pattern":
		s.collectReboundTargetNames(n.ChildByFieldName("value"), source)
	case "assignment_pattern":
		s.collectReboundTargetNames(n.ChildByFieldName("left"), source)
	case "rest_pattern":
		s.collectReboundTargetNames(n.ChildByFieldName("argument"), source)
	case "object_pattern", "array_pattern", "parenthesized_expression":
		count := n.ChildCount()
		for i := 0; i < count; i++ {
			s.collectReboundTargetNames(n.Child(i), source)
		}
	}
}
