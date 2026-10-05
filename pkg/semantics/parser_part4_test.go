package semantics

import (
	"context"
	"errors"
	"testing"
)

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

// AC-1.4: empty content must be rejected with an error matching
// ErrEmptyContent.
func TestValidate_RejectsEmptyContent(t *testing.T) {
	result, err := validate(context.Background(), []byte{}, LanguageGo, 0, nil)

	if result != nil {
		t.Errorf("AC-1.4: validate with empty content: got result %+v, want nil", result)
	}
	if !errors.Is(err, ErrEmptyContent) {
		t.Errorf("AC-1.4: validate with empty content: got err %v, want errors.Is(err, ErrEmptyContent)", err)
	}
}

// AC-1.5: any language other than LanguageGo must be rejected with an error
// matching ErrUnsupportedLanguage.
func TestValidate_RejectsUnsupportedLanguage(t *testing.T) {
	result, err := validate(context.Background(), []byte("package main\n"), Language("python"), 0, nil)

	if result != nil {
		t.Errorf("AC-1.5: validate with unsupported language: got result %+v, want nil", result)
	}
	if !errors.Is(err, ErrUnsupportedLanguage) {
		t.Errorf("AC-1.5: validate with unsupported language: got err %v, want errors.Is(err, ErrUnsupportedLanguage)", err)
	}
}
