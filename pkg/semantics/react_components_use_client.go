package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// moduleHasUseClientDirective reports whether root's first non-comment
// top-level statement is an expression_statement whose sole expression is
// the string literal "use client" (either quote style).
func moduleHasUseClientDirective(root engine.Node, source []byte) bool {
	count := root.ChildCount()
	for i := 0; i < count; i++ {
		c := root.Child(i)
		if c.Kind() == "comment" {
			continue
		}
		if c.Kind() != "expression_statement" {
			return false
		}
		str := reactFirstStringChild(c)
		if str == nil {
			return false
		}
		return reactIsUseClientLiteral(str, source)
	}
	return false
}

func reactFirstStringChild(n engine.Node) engine.Node {
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		if c := n.Child(i); c.Kind() == "string" {
			return c
		}
	}
	return nil
}

// reactIsUseClientLiteral reports whether str's raw source text is exactly
// "use client" or 'use client'. tsStringLiteralText is not reused here
// because it delegates to strconv.Unquote, which rejects JS single-quoted
// string literals (a Go-only quoting rule), silently dropping the
// single-quoted spelling of the directive.
func reactIsUseClientLiteral(str engine.Node, source []byte) bool {
	text := str.Utf8Text(source)
	if len(text) < 2 {
		return false
	}
	quote := text[0]
	if (quote != '"' && quote != '\'') || text[len(text)-1] != quote {
		return false
	}
	return text[1:len(text)-1] == "use client"
}
