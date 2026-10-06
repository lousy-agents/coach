package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func reactContainsJSX(n engine.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case "jsx_element", "jsx_self_closing_element", "jsx_fragment":
		return true
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		if reactContainsJSX(n.Child(i)) {
			return true
		}
	}
	return false
}

func reactJSXElementTagName(n engine.Node, source []byte) string {
	switch n.Kind() {
	case "jsx_element":
		open := n.ChildByFieldName("open_tag")
		if open == nil {
			return ""
		}
		if name := open.ChildByFieldName("name"); name != nil {
			return name.Utf8Text(source)
		}
		return ""
	case "jsx_self_closing_element":
		if name := n.ChildByFieldName("name"); name != nil {
			return name.Utf8Text(source)
		}
		return ""
	default:
		return ""
	}
}

func reactPascalCaseJSXTag(n engine.Node, source []byte) (string, bool) {
	switch n.Kind() {
	case "jsx_opening_element", "jsx_self_closing_element":
	default:
		return "", false
	}
	nameNode := n.ChildByFieldName("name")
	if nameNode == nil {
		return "", false
	}
	tag := nameNode.Utf8Text(source)
	if !isPascalCaseName(tag) {
		return "", false
	}
	return tag, true
}
