package semantics

import (
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

// Language identifies the source language a Result was produced from.
type Language string

// LanguageGo is the only supported Language in v1.
const LanguageGo Language = "go"

// ParseStatus summarizes whether Tree-sitter found any syntax errors while
// parsing the source. The only values that exist in v1 are "ok" and
// "syntax_errors".
type ParseStatus string

// languageSpec bundles everything the analyzer pipeline needs to parse and
// analyze one Language's source: which Tree-sitter grammar to load, and
// language-specific extraction logic for imports and structural
// metrics/findings.
//
// This is deliberately unexported and internal-only, not a public
// LanguageSupport interface: only one language (Go) exists to inform its
// shape. Adding a second language is expected to extend languageRegistry
// (and add its own extract*/compute* implementation, mirroring
// extractGoImports/computeGoFeatures) rather than change the pipeline code
// in parser.go or analyzer.go.
type languageSpec struct {
	// grammar loads the Tree-sitter grammar for this language, matching the
	// signature every tree-sitter-<language> binding exposes (e.g.
	// tree_sitter_go.Language).
	grammar func() unsafe.Pointer
	// extractImports extracts this language's import declarations from a
	// parsed, syntax-error-free tree.
	extractImports func(root *tree_sitter.Node, source []byte) ([]ImportFeature, error)
	// computeFeatures computes this language's structural metrics and
	// findings from a parsed, syntax-error-free tree.
	computeFeatures func(root *tree_sitter.Node, source []byte) (StructuralMetrics, []Finding)
}

// languageRegistry is the single source of truth for which Language values
// are supported and how to parse/analyze each one. NewAnalyzer and validate
// both check language support by membership in this map; syntaxParser.parse
// and Analyzer.AnalyzeBytes both dispatch to a language's extraction logic
// through the matching languageSpec.
var languageRegistry = map[Language]languageSpec{
	LanguageGo: {
		grammar:         tree_sitter_go.Language,
		extractImports:  extractGoImports,
		computeFeatures: computeGoFeatures,
	},
}
