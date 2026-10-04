package semantics

import (
	"context"
	"errors"
	"testing"
)

// AC-2.3: when syntax errors are detected, parseAndDetectSyntax must also
// return a non-nil error for which errors.Is(err, ErrSyntax) is true and
// errors.As(err, &target) succeeds for target *SyntaxError, carrying the
// same issues as the result's SyntaxErrors.
func TestSyntaxDetection_ReturnsErrorMatchingIsErrSyntaxAndAsSyntaxError(t *testing.T) {
	sp := newSyntaxParser()
	source := []byte("package main\nfunc {")

	result, err := sp.parseAndDetectSyntax(context.Background(), source, LanguageGo)
	if result == nil {
		t.Fatalf("parseAndDetectSyntax for source with a syntax error %q: got nil result, want a partial *Result", source)
	}

	if !errors.Is(err, ErrSyntax) {
		t.Errorf("parseAndDetectSyntax for source with a syntax error %q: errors.Is(err, ErrSyntax) = false, want true (err = %v)", source, err)
	}

	var syntaxErr *SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("parseAndDetectSyntax for source with a syntax error %q: errors.As(err, &SyntaxError{}) = false, want true (err = %v)", source, err)
	}

	if len(syntaxErr.Issues) != len(result.SyntaxErrors) {
		t.Fatalf("parseAndDetectSyntax for source with a syntax error %q: SyntaxError.Issues length = %d, want %d to match result.SyntaxErrors", source, len(syntaxErr.Issues), len(result.SyntaxErrors))
	}
	for i, want := range result.SyntaxErrors {
		if syntaxErr.Issues[i] != want {
			t.Errorf("parseAndDetectSyntax for source with a syntax error %q: SyntaxError.Issues[%d] = %+v, want %+v (must match result.SyntaxErrors)", source, i, syntaxErr.Issues[i], want)
		}
	}
}

// Supporting test (no AC number): parseAndDetectSyntax must report
// ParseStatus "ok" and a nil error for source with no syntax errors. This
// justifies the "clean tree" branch alongside the "syntax_errors" branch
// exercised by AC-2.1/2.2/2.4, so the function is a coherent pipeline stage
// rather than one that only handles the error path.
func TestParseAndDetectSyntax_ReturnsOkStatusForCleanSource(t *testing.T) {
	sp := newSyntaxParser()
	source := []byte("package main\nfunc main() {}\n")

	result, err := sp.parseAndDetectSyntax(context.Background(), source, LanguageGo)

	if err != nil {
		t.Fatalf("parseAndDetectSyntax for clean source %q: got err %v, want nil", source, err)
	}
	if result == nil {
		t.Fatalf("parseAndDetectSyntax for clean source %q: got nil result, want a non-nil *Result", source)
	}
	if result.ParseStatus != ParseStatus("ok") {
		t.Errorf("parseAndDetectSyntax for clean source %q: ParseStatus = %q, want %q", source, result.ParseStatus, "ok")
	}
	if len(result.SyntaxErrors) != 0 {
		t.Errorf("parseAndDetectSyntax for clean source %q: SyntaxErrors = %+v, want empty", source, result.SyntaxErrors)
	}
}
