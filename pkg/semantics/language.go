package semantics

import (
	"strings"
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

// Language identifies the source language a Result was produced from.
type Language string

// Supported Language values.
const (
	LanguageGo         Language = "go"
	LanguageTypeScript Language = "typescript"
	LanguageTSX        Language = "tsx"
)

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
// LanguageSupport interface. Adding a language extends languageRegistry
// (and adds its own extract*/compute* implementation, mirroring
// extractGoImports/computeGoFeatures or extractTSImports/computeTSFeatures)
// rather than changing the pipeline code in parser.go or analyzer.go.
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
	LanguageTypeScript: {
		grammar:         tree_sitter_typescript.LanguageTypescript,
		extractImports:  extractTSImports,
		computeFeatures: computeTSFeatures,
	},
	LanguageTSX: {
		grammar:         tree_sitter_typescript.LanguageTSX,
		extractImports:  extractTSXImports,
		computeFeatures: computeTSFeatures,
	},
}

// LanguageForExtension maps a file extension (including the leading dot, as
// returned by filepath.Ext, e.g. ".go") to the Language it corresponds to.
// Matching is case-insensitive. It returns ("", false) for any extension
// with no known mapping. This is an additive convenience for callers that
// want to select a Language from a file path; AnalyzeBytes itself never
// calls it and continues to route purely on the caller-supplied
// FileInput.Language.
func LanguageForExtension(ext string) (Language, bool) {
	switch strings.ToLower(ext) {
	case ".go":
		return LanguageGo, true
	case ".ts":
		return LanguageTypeScript, true
	case ".tsx":
		return LanguageTSX, true
	default:
		return "", false
	}
}
