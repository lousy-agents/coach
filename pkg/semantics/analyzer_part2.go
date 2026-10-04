package semantics

import (
	"fmt"
)

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
