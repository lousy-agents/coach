package semantics

import (
	"context"
	"fmt"
)

// AnalyzerOptions configures a new Analyzer.
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

// Analyzer runs the parse -> syntax-check -> extract pipeline against raw
// source bytes. It holds no C-backed resources between calls -- Parser,
// Tree, Query, and QueryCursor are all created fresh inside AnalyzeBytes --
// so a single *Analyzer is safe for concurrent use and has no Close method.
type Analyzer struct {
	maxFileBytes int
	languages    map[Language]bool // empty means "all supported"
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

// FileInput is one file to analyze. Path is opaque metadata: it is echoed
// into the returned Result verbatim (empty is allowed) but never opened.
type FileInput struct {
	Path     string
	Language Language
	Content  []byte
}

// AnalyzeBytes runs the full parse -> syntax-check -> extract pipeline
// against in.Content. On a clean parse it returns a Result with ParseStatus
// "ok" and a nil error. On a parse tree containing ERROR/MISSING nodes, it
// returns a partial Result (ParseStatus "syntax_errors", SyntaxErrors
// populated, Imports/Metrics/Findings zero-valued) alongside a non-nil error
// satisfying errors.Is(err, ErrSyntax). Any precondition failure (bad
// context, empty content, unsupported language, oversized content, binary
// content) or parse failure returns (nil, err).
func (a *Analyzer) AnalyzeBytes(ctx context.Context, in FileInput) (*Result, error) {
	if _, err := validate(ctx, in.Content, in.Language, a.maxFileBytes, a.languages); err != nil {
		return nil, err
	}

	sp := newSyntaxParser()
	tree, err := sp.parse(ctx, in.Content, in.Language)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	root := tree.RootNode()
	if root.HasError() {
		issues := collectSyntaxIssues(root)
		result := &Result{
			Path:         in.Path,
			Language:     in.Language,
			ParseStatus:  ParseStatus("syntax_errors"),
			SyntaxErrors: issues,
		}
		return result, &SyntaxError{Issues: issues}
	}

	// gotreesitter's TypeScript/TSX error recovery for a missing
	// right-hand-side expression (e.g. "const x = ;") discards the
	// malformed declaration entirely rather than emitting an ERROR/MISSING
	// node, so HasError() alone misses it (issue #33). This narrow,
	// TS/TSX-only fallback catches that specific shape when the grammar
	// itself reported a clean parse.
	if in.Language == LanguageTypeScript || in.Language == LanguageTSX {
		if issues := detectTSBareStatementTokens(root); len(issues) > 0 {
			result := &Result{
				Path:         in.Path,
				Language:     in.Language,
				ParseStatus:  ParseStatus("syntax_errors"),
				SyntaxErrors: issues,
			}
			return result, &SyntaxError{Issues: issues}
		}
	}

	// AC-1.8 (mid-pipeline): re-check cancellation between parsing and
	// running import/feature extraction, since validate and parse only
	// check at their own entry points.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// validate has already confirmed in.Language is registered, so this
	// lookup cannot miss.
	spec := languageRegistry[in.Language]
	imports, err := spec.extractImports(spec.engineLang, root, in.Content)
	if err != nil {
		return nil, err
	}
	metrics, findings := spec.computeFeatures(root, in.Content)

	result := &Result{
		Path:        in.Path,
		Language:    in.Language,
		ParseStatus: ParseStatus("ok"),
		Imports:     imports,
		Metrics:     metrics,
		Findings:    findings,
	}
	return result, nil
}
