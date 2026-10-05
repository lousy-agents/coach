package semantics

import (
	"context"
	"errors"
	"testing"
)

// AC-2.5: a zero-width MISSING node (Tree-sitter's error recovery inserting
// a virtual token, e.g. the missing ")" that closes an unterminated call
// argument list) must report Location.StartByte == Location.EndByte, with
// no error. This exercises real Tree-sitter output -- confirmed by
// experiment (see task notes) that "g(1, 2" with no closing paren produces
// exactly one MISSING ")" node at a single byte offset, no ERROR node.
func TestSyntaxDetection_MissingNodeReportsZeroWidthLocation(t *testing.T) {
	sp := newSyntaxParser()
	source := []byte("package main\nfunc f() {\n\tg(1, 2\n}\n")

	result, err := sp.parseAndDetectSyntax(context.Background(), source, LanguageGo)
	if result == nil {
		t.Fatalf("parseAndDetectSyntax for source with an unclosed call %q: got nil result, want a partial *Result", source)
	}
	if err == nil {
		t.Fatalf("parseAndDetectSyntax for source with an unclosed call %q: got nil err, want a non-nil syntax error", source)
	}

	var missing *SyntaxIssue
	for i := range result.SyntaxErrors {
		if result.SyntaxErrors[i].Kind == "missing" {
			missing = &result.SyntaxErrors[i]
			break
		}
	}
	if missing == nil {
		t.Fatalf("parseAndDetectSyntax for source with an unclosed call %q: SyntaxErrors = %+v, want at least one issue with Kind == %q", source, result.SyntaxErrors, "missing")
	}

	if missing.Location.StartByte != missing.Location.EndByte {
		t.Errorf("parseAndDetectSyntax for source with an unclosed call %q: MISSING node Location = %+v, want StartByte == EndByte (zero-width)", source, missing.Location)
	}
}

// Regression guard raised by review: language-support rejection (both "not
// registered at all" and "registered but outside this Analyzer's configured
// subset") must take precedence over the size check. Previously, the
// configured-subset check ran in AnalyzeBytes only after validate had
// already returned success, so an oversized file in a registered-but-
// unconfigured language misreported ErrFileTooLarge instead of
// ErrUnsupportedLanguage. allowed here excludes LanguageGo even though it is
// registered, so this locks in that validate itself -- not a caller running
// a second check afterward -- is the single place deciding both conditions,
// and does so before the size check.
func TestValidate_UnconfiguredLanguageTakesPrecedenceOverOversizedContent(t *testing.T) {
	const maxFileBytes = 10
	content := []byte("this is more than ten bytes")
	allowed := map[Language]bool{"other-lang": true}

	result, err := validate(context.Background(), content, LanguageGo, maxFileBytes, allowed)

	if result != nil {
		t.Errorf("validate with oversized content in an unconfigured language: got result %+v, want nil", result)
	}
	if !errors.Is(err, ErrUnsupportedLanguage) {
		t.Errorf("validate with oversized content in an unconfigured language: got err %v, want errors.Is(err, ErrUnsupportedLanguage)", err)
	}
	if errors.Is(err, ErrFileTooLarge) {
		t.Errorf("validate with oversized content in an unconfigured language: got err %v, want it NOT to match ErrFileTooLarge", err)
	}
}

// AC-1.8: if the supplied context is already cancelled, validate must report
// (nil, ctx.Err()) before doing any parsing work.
func TestValidate_RejectsAlreadyCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := validate(ctx, []byte("package main\n"), LanguageGo, 0, nil)

	if result != nil {
		t.Errorf("AC-1.8: validate with a cancelled context: got result %+v, want nil", result)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("AC-1.8: validate with a cancelled context: got err %v, want errors.Is(err, context.Canceled)", err)
	}
}
