package semantics

import "github.com/lousy-agents/coach/pkg/semantics/internal/engine"

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

// reactJSXAttributeValueNode returns attr's value node -- attr's child 2,
// either a bare string or a jsx_expression -- or nil when attr has no
// value (e.g. a boolean shorthand attribute).
func reactJSXAttributeValueNode(attr engine.Node) engine.Node {
	if attr.ChildCount() < 3 {
		return nil
	}
	return attr.Child(2)
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

// reactBareStringText strips n's quote characters (single or double),
// mirroring reactIsUseClientLiteral's quoting rule (strconv.Unquote
// rejects JS single-quoted strings, so it is not reused here).
func reactBareStringText(n engine.Node, source []byte) (string, bool) {
	if n == nil || n.Kind() != "string" {
		return "", false
	}
	text := n.Utf8Text(source)
	if len(text) < 2 {
		return "", false
	}
	quote := text[0]
	if (quote != '"' && quote != '\'') || text[len(text)-1] != quote {
		return "", false
	}
	return text[1 : len(text)-1], true
}
