package semantics

import (
	"strings"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
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
		if name, ok := firstChildIdentifier(n, source); ok {
			return map[string]bool{name: true}
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

func firstChildIdentifier(n engine.Node, source []byte) (string, bool) {
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		child := n.Child(i)
		if child.Kind() == "identifier" {
			return child.Utf8Text(source), true
		}
	}
	return "", false
}
