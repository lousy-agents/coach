package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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
func reactFirstStringChild(n engine.Node) engine.Node {
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		if c := n.Child(i); c.Kind() == "string" {
			return c
		}
	}
	return nil
}

// reactFuncOwnName returns n's own syntactic "name" field text, or "" when
// n has none (anonymous function_expression, or any arrow_function).
func reactFuncOwnName(n engine.Node, source []byte) string {
	if n == nil {
		return ""
	}
	if name := n.ChildByFieldName("name"); name != nil {
		return name.Utf8Text(source)
	}
	return ""
}
func reactFactsFromExport(exportStmt engine.Node, source []byte, hasDirective bool, bindings map[string]engine.Node) []ReactComponentFacts {
	var out []ReactComponentFacts
	for _, cand := range reactExportedCandidates(exportStmt, source, bindings) {
		rec, ok := reactBuildComponentFacts(cand, hasDirective, source)
		if !ok {
			continue
		}
		out = append(out, rec)
	}
	return out
}
func reactScopeContainsJSX(body engine.Node, source []byte) bool {
	found := false
	reactWalkScope(body, source, func(n engine.Node) {
		if found {
			return
		}
		switch n.Kind() {
		case "jsx_element", "jsx_self_closing_element", "jsx_fragment":
			found = true
		}
	})
	return found
}
