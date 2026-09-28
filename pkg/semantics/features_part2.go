package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"

	"strings"
)

func typeSwitchGuardIdentifiers(n engine.Node, source []byte) map[string]bool {
	if n == nil {
		return nil
	}
	if n.Kind() == "type_switch_guard" {
		text := n.Utf8Text(source)
		if !strings.Contains(text, ":=") {
			return nil
		}
		count := n.ChildCount()
		for i := 0; i < count; i++ {
			child := n.Child(i)
			if child.Kind() == "identifier" {
				return map[string]bool{child.Utf8Text(source): true}
			}
		}
		return nil
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		if names := typeSwitchGuardIdentifiers(n.Child(i), source); len(names) > 0 {
			return names
		}
	}
	return nil
}
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
