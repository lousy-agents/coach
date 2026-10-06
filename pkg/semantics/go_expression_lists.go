package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// goExpressionListValues returns list's non-punctuation children in source
// order (filtering out "," and any other literal tokens), i.e. the actual
// expression nodes an expression_list holds.
func goExpressionListValues(list engine.Node) []engine.Node {
	var out []engine.Node
	count := list.ChildCount()
	for i := 0; i < count; i++ {
		child := list.Child(i)
		if child.Kind() == "," {
			continue
		}
		out = append(out, child)
	}
	return out
}

func singleIdentifierName(list engine.Node, source []byte) string {
	if list == nil {
		return ""
	}
	var id string
	count := list.ChildCount()
	for i := 0; i < count; i++ {
		child := list.Child(i)
		switch child.Kind() {
		case ",":
			continue
		case "identifier":
			if id != "" {
				return ""
			}
			id = child.Utf8Text(source)
		default:
			return ""
		}
	}
	return id
}

func expressionListIsSingleNode(list engine.Node, want engine.Node) bool {
	if list == nil || want == nil {
		return false
	}
	var found engine.Node
	count := list.ChildCount()
	for i := 0; i < count; i++ {
		child := list.Child(i)
		if child.Kind() == "," {
			continue
		}
		if found != nil {
			return false
		}
		found = child
	}
	return found != nil && sameNodeSpan(found, want)
}
