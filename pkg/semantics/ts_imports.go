package semantics

import (
	"fmt"
	"sort"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// tsImportQuerySource finds the module-specifier string of every static
// import declaration (`import ... from "..."` and side-effect-only
// `import "..."`). It deliberately does not match `require(...)`, dynamic
// `import(...)`, or `export ... from "..."` re-exports (out of scope for
// v1, D4/AC-R3.3).
const tsImportQuerySource = `(import_statement source: (string) @import.path)`

// extractTSImports finds every static import module specifier under root
// (a TypeScript-grammar tree) using tsImportQuerySource, and returns one
// ImportFeature per specifier found, ordered by Location.StartByte
// ascending. Alias is always "" for TS imports in v1 (D4): TS import
// bindings (named/default/namespace) are out of scope.
func extractTSImports(lang engine.Language, root engine.Node, source []byte) ([]ImportFeature, error) {
	return extractTSImportsForGrammar(lang, root, source)
}

// extractTSXImports is extractTSImports's TSX-grammar counterpart. A
// Tree-sitter Query is compiled against, and only matches node-type IDs
// from, the specific Language it was built with: although the TypeScript
// and TSX grammars share identical node *kind name strings* (confirmed by
// TestTSImportQuerySource_MatchesTSXGrammarNodeIDs), their internal type
// IDs differ, so a query compiled against one grammar silently matches
// nothing against a tree parsed with the other. extractImports therefore
// cannot be a single function shared byte-for-byte across both languageSpec
// entries the way computeFeatures is (computeTSFeatures matches on
// Node.Kind() strings alone, which every Node resolves against its own
// tree's language and so needs no query); it is shared logic parameterized
// by grammar instead -- here, by which engine.Language the caller passes.
func extractTSXImports(lang engine.Language, root engine.Node, source []byte) ([]ImportFeature, error) {
	return extractTSImportsForGrammar(lang, root, source)
}

// extractTSImportsForGrammar is the grammar-parameterized implementation
// shared by extractTSImports and extractTSXImports.
func extractTSImportsForGrammar(lang engine.Language, root engine.Node, source []byte) ([]ImportFeature, error) {
	query, queryErr := lang.NewQuery(tsImportQuerySource)
	if queryErr != nil {
		return nil, fmt.Errorf("semantics: compiling TS import query: %w", queryErr)
	}
	defer query.Close()

	cursor := lang.NewQueryCursor()
	defer cursor.Close()

	var imports []ImportFeature
	matches := cursor.Matches(query, root, source)
	for {
		match := matches.Next()
		if match == nil {
			break
		}
		for _, capture := range match.Captures {
			pathNode := capture.Node
			imports = append(imports, ImportFeature{
				Path:     tsUnquoteString(pathNode.Utf8Text(source)),
				Location: locationFromNode(pathNode),
			})
		}
	}

	sort.Slice(imports, func(i, j int) bool {
		return imports[i].Location.StartByte < imports[j].Location.StartByte
	})

	return imports, nil
}

// tsUnquoteString strips one matching leading/trailing quote character
// (a single quote or a double quote) from raw, the verbatim source text of
// a Tree-sitter
// `(string)` node. TS module specifiers may be single- or double-quoted, so
// this uses explicit delimiter stripping rather than strconv.Unquote (which
// rejects single-quoted strings) (D4). raw is already known to be a
// well-formed quoted string literal by the time this runs (the tree is
// syntax-error-free), so no escape-sequence interpretation is attempted.
func tsUnquoteString(raw string) string {
	if len(raw) < 2 {
		return raw
	}
	first, last := raw[0], raw[len(raw)-1]
	if (first == '\'' || first == '"') && first == last {
		return raw[1 : len(raw)-1]
	}
	return raw
}
