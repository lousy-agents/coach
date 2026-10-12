package semantics

import (
	"context"
	"testing"
)

// errAfterNCallsContext wraps a context.Context, overriding only Err() so it
// reports nil for the first n calls and context.Canceled thereafter.
type errAfterNCallsContext struct {
	context.Context
	n     int
	calls int
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
