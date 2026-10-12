package semantics

import (
	"context"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// mustParseGo parses source as Go and returns its root node plus a cleanup
// function that closes the underlying tree. It reuses the syntaxParser seam
// already exercised by parser_test.go rather than duplicating Tree-sitter
// setup here.
func mustParseGo(t *testing.T, source []byte) (engine.Node, func()) {
	t.Helper()

	sp := newSyntaxParser()
	tree, err := sp.parse(context.Background(), source, LanguageGo)
	if err != nil {
		t.Fatalf("parsing Go source %q: got err %v, want nil", source, err)
	}
	return tree.RootNode(), tree.Close
}

// hasFinding reports whether findings contains a Finding with the given
// kind and name.
func hasFinding(findings []Finding, kind, name string) bool {
	for _, f := range findings {
		if f.Kind == kind && f.Name == name {
			return true
		}
	}
	return false
}

// mutatesInputFinding returns a pointer to the first "mutates_input" Finding
// in findings matching name, or nil if there is none.
func mutatesInputFinding(findings []Finding, name string) *Finding {
	for i := range findings {
		if findings[i].Kind == "mutates_input" && findings[i].Name == name {
			return &findings[i]
		}
	}
	return nil
}

// countMutatesInputFindings reports how many "mutates_input" findings with
// the given name are present in findings.
func countMutatesInputFindings(findings []Finding, name string) int {
	n := 0
	for _, f := range findings {
		if f.Kind == "mutates_input" && f.Name == name {
			n++
		}
	}
	return n
}
