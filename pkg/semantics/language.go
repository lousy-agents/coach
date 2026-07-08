package semantics

import (
	"strings"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
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
// analyze one Language's source: a backend-bound grammar handle, and
// language-specific extraction logic for imports and structural
// metrics/findings.
//
// This is deliberately unexported and internal-only, not a public
// LanguageSupport interface. Adding a language extends languageRegistry
// (and adds its own extract*/compute* implementation, mirroring
// extractGoImports/computeGoFeatures or extractTSImports/computeTSFeatures)
// rather than changing the pipeline code in parser.go or analyzer.go.
//
// languageRegistry itself -- the map[Language]languageSpec populating these
// fields -- lives in language_cgo.go or language_gotreesitter.go, whichever
// is compiled in for a given build (see those files' build tags): exactly
// one of them is ever part of any single binary, so there is exactly one
// languageRegistry.
type languageSpec struct {
	// engineLang is the backend-bound grammar handle for this language.
	engineLang engine.Language
	// extractImports extracts this language's import declarations from a
	// parsed, syntax-error-free tree.
	extractImports func(lang engine.Language, root engine.Node, source []byte) ([]ImportFeature, error)
	// computeFeatures computes this language's structural metrics and
	// findings from a parsed, syntax-error-free tree.
	computeFeatures func(root engine.Node, source []byte) (StructuralMetrics, []Finding)
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
