package semantics

import (
	"context"

	"errors"

	"testing"
)

// errAfterNCallsContext wraps a context.Context, overriding only Err() so it
// reports nil for the first n calls and context.Canceled thereafter.
type errAfterNCallsContext struct {
	context.Context
	n     int
	calls int
}

// AC-2.2 / AC-2.3 end-to-end: through the full public AnalyzeBytes call, a
// source with a syntax error must produce a partial Result (ParseStatus
// "syntax_errors", Imports/Findings empty, Metrics zero) alongside an error
// that satisfies errors.Is(err, ErrSyntax) and errors.As(err, &SyntaxError)
// with Issues matching result.SyntaxErrors exactly -- the same contract
// parseAndDetectSyntax already proved at the syntaxParser level (Task 3),
// now surfaced through the public facade.
func TestAnalyzeBytes_EndToEndSyntaxErrorContract(t *testing.T) {
	a := mustNewAnalyzer(t)
	source := []byte("package main\nfunc {")

	result, err := a.AnalyzeBytes(context.Background(), FileInput{
		Path:     "broken.go",
		Language: LanguageGo,
		Content:  source,
	})

	if result == nil {
		t.Fatalf("AnalyzeBytes for source with a syntax error %q: got nil result, want a partial *Result", source)
	}
	if result.ParseStatus != ParseStatus("syntax_errors") {
		t.Errorf("AnalyzeBytes for source with a syntax error %q: ParseStatus = %q, want %q", source, result.ParseStatus, "syntax_errors")
	}
	if len(result.Imports) != 0 {
		t.Errorf("AnalyzeBytes for source with a syntax error %q: Imports = %+v, want empty", source, result.Imports)
	}
	if len(result.Findings) != 0 {
		t.Errorf("AnalyzeBytes for source with a syntax error %q: Findings = %+v, want empty", source, result.Findings)
	}
	if result.Metrics != (StructuralMetrics{}) {
		t.Errorf("AnalyzeBytes for source with a syntax error %q: Metrics = %+v, want the zero value", source, result.Metrics)
	}

	if !errors.Is(err, ErrSyntax) {
		t.Errorf("AnalyzeBytes for source with a syntax error %q: errors.Is(err, ErrSyntax) = false, want true (err = %v)", source, err)
	}

	var syntaxErr *SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("AnalyzeBytes for source with a syntax error %q: errors.As(err, &SyntaxError{}) = false, want true (err = %v)", source, err)
	}
	if len(syntaxErr.Issues) != len(result.SyntaxErrors) {
		t.Fatalf("AnalyzeBytes for source with a syntax error %q: SyntaxError.Issues length = %d, want %d to match result.SyntaxErrors", source, len(syntaxErr.Issues), len(result.SyntaxErrors))
	}
	for i, want := range result.SyntaxErrors {
		if syntaxErr.Issues[i] != want {
			t.Errorf("AnalyzeBytes for source with a syntax error %q: SyntaxError.Issues[%d] = %+v, want %+v (must match result.SyntaxErrors)", source, i, syntaxErr.Issues[i], want)
		}
	}
}

func (c *errAfterNCallsContext) Err() error {
	c.calls++
	if c.calls > c.n {
		return context.Canceled
	}
	return nil
}

// givenContextCanceledAfter returns a context that reports nil from Err()
// for the first n calls, then context.Canceled -- isolating exactly the
// cancellation check under test (labeled by where) from earlier checks in
// a multi-stage pipeline that would otherwise observe the cancellation
// first.
func givenContextCanceledAfter(n int, where string) context.Context {
	return &errAfterNCallsContext{Context: context.Background(), n: n}
}

// AC-1.8 (mid-pipeline): AnalyzeBytes must re-check ctx.Err() between
// parsing and running import/feature extraction, not just at validate's
// entry check.
//
// The context passed in reports nil for its first two Err() calls (one
// inside validate, one inside syntaxParser.parse -- both pass on this
// valid, clean-parsing source) and context.Canceled from the third call
// onward, so only AnalyzeBytes's own mid-pipeline check can observe the
// cancellation: if that check were missing, this test would see a
// successful result instead.
func TestAnalyzeBytes_ChecksCancellationBetweenParseAndFeatureExtraction(t *testing.T) {
	a := mustNewAnalyzer(t)
	ctx := givenContextCanceledAfter(2, "validate's and syntaxParser.parse's own ctx.Err() checks")
	source := []byte("package main\nfunc main() {}\n")

	result, err := a.AnalyzeBytes(ctx, FileInput{
		Path:     "main.go",
		Language: LanguageGo,
		Content:  source,
	})

	thenResultIsNil(t, result, "AnalyzeBytes cancelled between parse and feature extraction")
	thenErrorIs(t, err, context.Canceled, "AnalyzeBytes cancelled between parse and feature extraction")
}
