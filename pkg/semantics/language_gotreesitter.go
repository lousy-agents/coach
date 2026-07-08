//go:build !cgo || coach_gotreesitter

package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// languageRegistry is the single source of truth for which Language values
// are supported and how to parse/analyze each one, backed by the pure-Go
// gotreesitter engine (see pkg/semantics/internal/engine/gotreesitter.go).
// This compiles in automatically whenever CGO is unavailable (GOOS=js
// GOARCH=wasm, or any CGO_ENABLED=0 build -- exactly where
// language_cgo.go's CGO-based registry cannot compile at all), or when
// explicitly requested on a native build via the coach_gotreesitter build
// tag, e.g. for the conformance suite.
var languageRegistry = map[Language]languageSpec{
	LanguageGo: {
		engineLang:      engine.GoTreeSitterLanguage("go"),
		extractImports:  extractGoImports,
		computeFeatures: computeGoFeatures,
	},
	LanguageTypeScript: {
		engineLang:      engine.GoTreeSitterLanguage("typescript"),
		extractImports:  extractTSImports,
		computeFeatures: computeTSFeatures,
	},
	LanguageTSX: {
		engineLang:      engine.GoTreeSitterLanguage("tsx"),
		extractImports:  extractTSXImports,
		computeFeatures: computeTSFeatures,
	},
}
