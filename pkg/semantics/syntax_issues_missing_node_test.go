package semantics

import (
	"context"
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
