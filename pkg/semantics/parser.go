package semantics

import (
	"bytes"
	"context"
	"fmt"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// defaultMaxFileBytes is the content size limit applied when the caller
// specifies maxFileBytes <= 0 (e.g. the zero value).
const defaultMaxFileBytes = 2 * 1024 * 1024 // 2 MiB

// validate applies precondition checks to content before parsing, in the
// order: context cancellation, emptiness, language support, size limit, then
// binary (NUL byte) detection. The *Result return is always nil; it exists
// so callers (e.g. the AnalyzeBytes facade) can propagate (result, err)
// uniformly without a separate branch for validation failures.
//
// Language support is decided in exactly one place and one step: lang must
// both be registered in languageRegistry (the same registry NewAnalyzer
// validates AnalyzerOptions.Languages against) and, when allowed is
// non-empty, be a member of allowed (an Analyzer's configured language
// subset). allowed being empty means "any registered language is allowed."
// Deciding both conditions before the size check -- rather than checking the
// configured subset separately after validate returns -- prevents an
// oversized file in an unconfigured language from misreporting
// ErrFileTooLarge instead of ErrUnsupportedLanguage.
func validate(ctx context.Context, content []byte, lang Language, maxFileBytes int, allowed map[Language]bool) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(content) == 0 {
		return nil, ErrEmptyContent
	}
	if _, ok := languageRegistry[lang]; !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedLanguage, lang)
	}
	if len(allowed) > 0 && !allowed[lang] {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedLanguage, lang)
	}
	max := maxFileBytes
	if max <= 0 {
		max = defaultMaxFileBytes
	}
	if len(content) > max {
		return nil, fmt.Errorf("%w: content is %d bytes, exceeds max %d bytes", ErrFileTooLarge, len(content), max)
	}
	if bytes.IndexByte(content, 0x00) != -1 {
		return nil, ErrBinaryContent
	}
	return nil, nil
}

// syntaxParser runs the Tree-sitter parse stage of the pipeline. Its parse
// field wraps the actual engine.Parser.Parse call behind a function field
// so tests can force a nil-tree return (AC-6.2), a case that is not
// otherwise reachable through normal Parser.Parse calls with valid input.
// The zero value is not usable; construct with newSyntaxParser.
type syntaxParser struct {
	parseFunc func(p engine.Parser, content []byte) (engine.Tree, error)
}

func newSyntaxParser() *syntaxParser {
	return &syntaxParser{
		parseFunc: func(p engine.Parser, content []byte) (engine.Tree, error) {
			return p.Parse(content)
		},
	}
}

// parse creates a per-call Parser configured for lang's grammar (looked up
// in languageRegistry), parses content, and returns the resulting Tree. The
// caller owns the returned tree and must Close it. If the context is
// already cancelled, parse returns (nil, ctx.Err()) directly (e.g.
// context.Canceled), matching validate's own cancellation check rather than
// wrapping it in ErrParseFailure. If lang is not registered or the parser
// fails to produce a tree, parse returns an error satisfying
// errors.Is(err, ErrParseFailure).
func (sp *syntaxParser) parse(ctx context.Context, content []byte, lang Language) (engine.Tree, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	spec, ok := languageRegistry[lang]
	if !ok {
		return nil, fmt.Errorf("%w: %q has no registered grammar", ErrParseFailure, lang)
	}

	parser, err := spec.engineLang.NewParser()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrParseFailure, err)
	}

	tree, err := sp.parseFunc(parser, content)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrParseFailure, err)
	}
	if tree == nil {
		return nil, fmt.Errorf("%w: Parse returned a nil tree", ErrParseFailure)
	}
	return tree, nil
}

// collectSyntaxIssues walks the tree rooted at n in pre-order and collects a
// SyntaxIssue for every ERROR or MISSING node found.
func collectSyntaxIssues(n engine.Node) []SyntaxIssue {
	var issues []SyntaxIssue
	var walk func(node engine.Node)
	walk = func(node engine.Node) {
		if node == nil {
			return
		}
		switch {
		case node.IsMissing():
			issues = append(issues, SyntaxIssue{Kind: "missing", Location: locationFromNode(node)})
		case node.IsError():
			issues = append(issues, SyntaxIssue{Kind: "error", Location: locationFromNode(node)})
		}
		count := node.ChildCount()
		for i := 0; i < count; i++ {
			walk(node.Child(i))
		}
	}
	walk(n)
	return issues
}

// locationFromNode converts a node's span into our Location type,
// preserving Tree-sitter's 0-based byte/row/col values verbatim.
func locationFromNode(n engine.Node) Location {
	startRow, startCol := n.StartPoint()
	endRow, endCol := n.EndPoint()
	return Location{
		StartByte: n.StartByte(),
		EndByte:   n.EndByte(),
		StartRow:  startRow,
		StartCol:  startCol,
		EndRow:    endRow,
		EndCol:    endCol,
	}
}
