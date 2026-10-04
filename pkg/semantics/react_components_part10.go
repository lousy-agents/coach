package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// reactArrayPatternElements returns n's elements by position, including a
// nil placeholder for an elided element (e.g. `[, setC]`), so that index 0
// is always the binding slot and index 1 the setter slot even when earlier
// elements are holes.
func reactArrayPatternElements(n engine.Node) []engine.Node {
	var out []engine.Node
	count := n.ChildCount()
	pendingElement := false
	for i := 0; i < count; i++ {
		c := n.Child(i)
		switch c.Kind() {
		case "[", "]":
			continue
		case ",":
			out = reactArrayPatternOnComma(out, pendingElement)
			pendingElement = false
		default:
			out = append(out, c)
			pendingElement = true
		}
	}
	return out
}
func reactJSXAttributeNameText(attr engine.Node, source []byte) string {
	if attr.ChildCount() == 0 {
		return ""
	}
	nameNode := attr.Child(0)
	if nameNode.Kind() != "property_identifier" {
		return ""
	}
	return nameNode.Utf8Text(source)
}
func reactJSXAttributes(n engine.Node) []engine.Node {
	var out []engine.Node
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		if c := n.Child(i); c.Kind() == "jsx_attribute" {
			out = append(out, c)
		}
	}
	return out
}

// reactJSXExpressionInner returns the expression node wrapped by a
// jsx_expression ("{" expr "}"), skipping the brace tokens, or nil when
// none is present.
func reactJSXExpressionInner(jsxExpr engine.Node) engine.Node {
	count := jsxExpr.ChildCount()
	for i := 0; i < count; i++ {
		c := jsxExpr.Child(i)
		switch c.Kind() {
		case "{", "}":
			continue
		default:
			return c
		}
	}
	return nil
}
func reactDedupeComponentsBySpan(in []ReactComponentFacts) []ReactComponentFacts {
	seen := map[[2]uint]struct{}{}
	var out []ReactComponentFacts
	for _, rec := range in {
		span := [2]uint{rec.Location.StartByte, rec.Location.EndByte}
		if _, dup := seen[span]; dup {
			continue
		}
		seen[span] = struct{}{}
		out = append(out, rec)
	}
	return out
}
