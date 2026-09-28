package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func (s nameSet) collectBindingPatternNames(n engine.Node, source []byte) {
	if n == nil {
		return
	}
	switch n.Kind() {
	case "identifier", "shorthand_property_identifier_pattern":
		s[n.Utf8Text(source)] = true
		return
	case "pair_pattern":
		if value := n.ChildByFieldName("value"); value != nil {
			s.collectBindingPatternNames(value, source)
		}
		return
	case "rest_pattern":
		if arg := n.ChildByFieldName("argument"); arg != nil {
			s.collectBindingPatternNames(arg, source)
		}
		return
	case "assignment_pattern":
		if left := n.ChildByFieldName("left"); left != nil {
			s.collectBindingPatternNames(left, source)
		}
		return
	default:
		count := n.ChildCount()
		for i := 0; i < count; i++ {
			s.collectBindingPatternNames(n.Child(i), source)
		}
	}
}
func tsCurrentFunctionParamNames(scopes []tsParamScope) map[string]bool {
	for i := len(scopes) - 1; i >= 0; i-- {
		if scopes[i].ownerName == "" {
			continue
		}
		return paramNamesInScope(scopes[i].bindings)
	}
	return nil
}

func paramNamesInScope(bindings map[string]bool) map[string]bool {
	names := map[string]bool{}
	for name, isParam := range bindings {
		if isParam {
			names[name] = true
		}
	}
	return names
}
