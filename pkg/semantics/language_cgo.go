//go:build cgo && !coach_gotreesitter

package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

// languageRegistry is the single source of truth for which Language values
// are supported and how to parse/analyze each one, backed by the CGO
// Tree-sitter engine (see pkg/semantics/internal/engine/cgo.go). This is
// the default on any platform where CGO is available; see
// language_gotreesitter.go for the pure-Go fallback used when it is not
// (GOOS=js GOARCH=wasm, or any CGO_ENABLED=0 build), or when explicitly
// requested via the coach_gotreesitter build tag.
var languageRegistry = map[Language]languageSpec{
	LanguageGo: {
		engineLang:      engine.CGOLanguage(tree_sitter_go.Language),
		extractImports:  extractGoImports,
		computeFeatures: computeGoFeatures,
	},
	LanguageTypeScript: {
		engineLang:      engine.CGOLanguage(tree_sitter_typescript.LanguageTypescript),
		extractImports:  extractTSImports,
		computeFeatures: computeTSFeatures,
	},
	LanguageTSX: {
		engineLang:      engine.CGOLanguage(tree_sitter_typescript.LanguageTSX),
		extractImports:  extractTSXImports,
		computeFeatures: computeTSFeatures,
	},
}
