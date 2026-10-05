package semantics

import (
	"context"
	"errors"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// Precondition for the syntax-detection tests below: parsing valid Go
// source through our own wrapper must produce a clean tree (no ERROR or
// MISSING nodes), the same as the raw Tree-sitter smoke test already shows.
func TestParse_ReturnsCleanTreeForValidSource(t *testing.T) {
	sp := newSyntaxParser()
	source := []byte("package main\nfunc main() {}\n")

	tree, err := sp.parse(context.Background(), source, LanguageGo)
	if err != nil {
		t.Fatalf("parse of valid source %q: got err %v, want nil", source, err)
	}
	if tree == nil {
		t.Fatalf("parse of valid source %q: got nil tree, want a non-nil tree", source)
	}
	defer tree.Close()

	if tree.RootNode().HasError() {
		t.Errorf("parse of valid source %q: RootNode().HasError() = true, want false", source)
	}
}

// Regression guard raised by review: parse's doc comment claimed a
// cancelled context returns an error matching ErrParseFailure, but the
// implementation returns ctx.Err() directly, matching validate's own
// cancellation check. This pins down the actual behavior with a direct
// test rather than leaving it only indirectly covered through validate and
// AnalyzeBytes.
func TestParse_ReturnsContextErrDirectlyOnAlreadyCancelledContext(t *testing.T) {
	sp := newSyntaxParser()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tree, err := sp.parse(ctx, []byte("package main\n"), LanguageGo)

	if tree != nil {
		t.Errorf("parse with an already-cancelled context: got non-nil tree, want nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("parse with an already-cancelled context: got err %v, want errors.Is(err, context.Canceled)", err)
	}
	if errors.Is(err, ErrParseFailure) {
		t.Errorf("parse with an already-cancelled context: got errors.Is(err, ErrParseFailure) = true, want false (cancellation is not a parse failure)")
	}
}

// AC-6.2: if the underlying Tree-sitter Parse call returns a nil tree, parse
// must report an error matching ErrParseFailure and must not dereference or
// Close the nil tree. A real nil tree isn't reachable through normal
// Parser.Parse calls with valid parser/content, so this test injects a
// forced-nil parseFunc via the syntaxParser seam built for this purpose.
func TestParse_ReturnsParseFailureErrorOnNilTreeWithoutDereferencing(t *testing.T) {
	sp := newSyntaxParser()
	sp.parseFunc = func(p engine.Parser, content []byte) (engine.Tree, error) {
		return nil, nil
	}

	tree, err := sp.parse(context.Background(), []byte("package main\n"), LanguageGo)

	if tree != nil {
		t.Fatalf("parse when the underlying Parse call returns nil: got non-nil tree %+v, want nil", tree)
	}
	if !errors.Is(err, ErrParseFailure) {
		t.Errorf("parse when the underlying Parse call returns nil: got err %v, want errors.Is(err, ErrParseFailure)", err)
	}
}
