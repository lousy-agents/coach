package semantics

import (
	"context"
	"errors"
	"testing"
)

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
