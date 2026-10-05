package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func reactPrimaryJSXElement(n engine.Node) engine.Node {
	if n == nil {
		return nil
	}
	switch n.Kind() {
	case "jsx_element", "jsx_self_closing_element":
		return n
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		if el := reactPrimaryJSXElement(n.Child(i)); el != nil {
			return el
		}
	}
	return nil
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
func reactLiteralLabelText(n engine.Node, source []byte) (string, bool) {
	if n == nil {
		return "", false
	}
	switch n.Kind() {
	case "string":
		return reactBareStringText(n, source)
	case "number":
		return n.Utf8Text(source), true
	default:
		return "", false
	}
}
