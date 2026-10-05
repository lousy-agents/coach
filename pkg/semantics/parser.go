package semantics

import (
	"context"
	"fmt"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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
