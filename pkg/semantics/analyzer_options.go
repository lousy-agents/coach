package semantics

import (
	"fmt"
)

type AnalyzerOptions struct {
	// Languages restricts AnalyzeBytes to this set of grammars. Empty means
	// "all supported" (LanguageGo, LanguageTypeScript, and LanguageTSX).
	// Any entry that is not a recognized Language makes NewAnalyzer return
	// an error.
	Languages []Language
	// MaxFileBytes caps the size of content AnalyzeBytes will parse. 0 uses
	// the package default (2 MiB); negative values make NewAnalyzer return
	// an error.
	MaxFileBytes int
}

// NewAnalyzer constructs an Analyzer from opts, validating that every
// requested language is recognized and that MaxFileBytes is non-negative.
func NewAnalyzer(opts AnalyzerOptions) (*Analyzer, error) {
	if opts.MaxFileBytes < 0 {
		return nil, fmt.Errorf("semantics: MaxFileBytes must be >= 0, got %d", opts.MaxFileBytes)
	}

	languages := make(map[Language]bool, len(opts.Languages))
	for _, lang := range opts.Languages {
		if _, ok := languageRegistry[lang]; !ok {
			return nil, fmt.Errorf("%w: %q", ErrUnsupportedLanguage, lang)
		}
		languages[lang] = true
	}

	return &Analyzer{
		maxFileBytes: opts.MaxFileBytes,
		languages:    languages,
	}, nil
}
