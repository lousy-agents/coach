package semantics

import (
	"context"
	"errors"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// AC-1.6: content exceeding the configured max size must be rejected with an
// error matching ErrFileTooLarge. Uses a small explicit max (10 bytes)
// rather than the real 2 MiB default so the test fixture stays tiny.
func TestValidate_RejectsContentOverMaxFileBytes(t *testing.T) {
	const maxFileBytes = 10
	content := []byte("this is more than ten bytes")

	result, err := validate(context.Background(), content, LanguageGo, maxFileBytes, nil)

	if result != nil {
		t.Errorf("AC-1.6: validate with content over max size: got result %+v, want nil", result)
	}
	if !errors.Is(err, ErrFileTooLarge) {
		t.Errorf("AC-1.6: validate with %d-byte content and max %d: got err %v, want errors.Is(err, ErrFileTooLarge)", len(content), maxFileBytes, err)
	}
}

// AC-1.7: content containing a NUL byte must be rejected with an error
// matching ErrBinaryContent.
func TestValidate_RejectsContentContainingNULByte(t *testing.T) {
	content := []byte("package main\x00\n")

	result, err := validate(context.Background(), content, LanguageGo, 0, nil)

	if result != nil {
		t.Errorf("AC-1.7: validate with a NUL byte in content: got result %+v, want nil", result)
	}
	if !errors.Is(err, ErrBinaryContent) {
		t.Errorf("AC-1.7: validate with a NUL byte in content: got err %v, want errors.Is(err, ErrBinaryContent)", err)
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
