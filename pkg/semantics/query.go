package semantics

import (
	"fmt"
	"sort"
	"strconv"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

// importQuerySource is the Tree-sitter query used to find import paths in
// both single (`import "fmt"`) and grouped (`import (...)`) import
// declarations. The capture name "import.path" is looked up via
// Query.CaptureNames() when iterating matches.
const importQuerySource = `(import_spec path: (_) @import.path)`

// extractImports finds every import path under root (single or grouped
// import declarations) using importQuerySource, and returns one
// ImportFeature per path found, ordered by Location.StartByte ascending
// (AC-1.10). Path is the interpreted string value (via strconv.Unquote),
// not the raw delimited source text, so escape sequences are resolved the
// same way the Go compiler resolves them.
func extractImports(root *tree_sitter.Node, source []byte) ([]ImportFeature, error) {
	lang := tree_sitter.NewLanguage(tree_sitter_go.Language())
	query, queryErr := tree_sitter.NewQuery(lang, importQuerySource)
	if queryErr != nil {
		return nil, fmt.Errorf("semantics: compiling import query: %w", queryErr)
	}
	defer query.Close()

	cursor := tree_sitter.NewQueryCursor()
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
			path, unquoteErr := strconv.Unquote(pathNode.Utf8Text(source))
			if unquoteErr != nil {
				// The tree is already known to be free of syntax errors by
				// the time extractImports runs (see AC-2.1's HasError gate),
				// so every import_spec path node's text is a valid Go string
				// literal; strconv.Unquote failing here means our
				// grammar/query assumptions are wrong, not that the source
				// is malformed. Surface it rather than silently falling back
				// to naive delimiter stripping, which would mask the bug by
				// returning wrong data instead of an error.
				return nil, fmt.Errorf("semantics: unquoting import path %q: %w", pathNode.Utf8Text(source), unquoteErr)
			}
			imports = append(imports, ImportFeature{
				Path:     path,
				Alias:    importAlias(&pathNode, source),
				Location: locationFromNode(&pathNode),
			})
		}
	}

	sort.Slice(imports, func(i, j int) bool {
		return imports[i].Location.StartByte < imports[j].Location.StartByte
	})

	return imports, nil
}

// importAlias reads the alias token from pathNode's parent import_spec's
// "name" field, if present: an identifier for `f "fmt"`, "." for dot
// imports, "_" for blank imports, or "" when the import has no alias.
func importAlias(pathNode *tree_sitter.Node, source []byte) string {
	spec := pathNode.Parent()
	if spec == nil {
		return ""
	}
	name := spec.ChildByFieldName("name")
	if name == nil {
		return ""
	}
	return name.Utf8Text(source)
}
