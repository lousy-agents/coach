package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"

	"unicode"
	"unicode/utf8"
)

func reactCollectExportedComponents(root engine.Node, source []byte, hasDirective bool, bindings map[string]engine.Node) []ReactComponentFacts {
	var out []ReactComponentFacts
	for _, exportStmt := range reactExportStatements(root) {
		out = append(out, reactFactsFromExport(exportStmt, source, hasDirective, bindings)...)
	}
	return reactDedupeComponentsBySpan(out)
}
func reactArrayPatternOnComma(out []engine.Node, pendingElement bool) []engine.Node {
	if pendingElement {
		return out
	}
	return append(out, nil)
}
func reactSingleCandidate(funcNode engine.Node, name string) []reactCandidate {
	if funcNode == nil {
		return nil
	}
	return []reactCandidate{{funcNode: funcNode, name: name}}
}
func isPascalCaseName(name string) bool {
	if name == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}
