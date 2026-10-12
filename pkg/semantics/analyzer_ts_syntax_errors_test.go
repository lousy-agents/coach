package semantics

import (
	"context"
	"errors"
	"testing"
)

// AC-R5.1, AC-R5.2: malformed TS must produce the same partial-Result +
// errors.Is(err, ErrSyntax) + errors.As(err, &SyntaxError) contract Go gets.
// For this particular source, gotreesitter's own root.HasError() reports a
// clean parse (issue #33), so the contract here is actually satisfied by
// detectTSBareStatementTokens's fallback, not by collectSyntaxIssues/
// HasError; see TestAnalyzeBytes_TSMissingInitializerFalseNegative for
// table-driven coverage of that fallback specifically.
func TestAnalyzeBytes_TSEndToEndSyntaxErrorContract(t *testing.T) {
	a := mustNewAnalyzer(t)
	source := []byte("const x = ;")

	result, err := a.AnalyzeBytes(context.Background(), FileInput{
		Path:     "broken.ts",
		Language: LanguageTypeScript,
		Content:  source,
	})

	if result == nil {
		t.Fatalf("AnalyzeBytes for TS source with a syntax error %q: got nil result, want a partial *Result", source)
	}
	if result.ParseStatus != ParseStatus("syntax_errors") {
		t.Errorf("AnalyzeBytes for TS source with a syntax error %q: ParseStatus = %q, want %q", source, result.ParseStatus, "syntax_errors")
	}
	if len(result.SyntaxErrors) == 0 {
		t.Errorf("AnalyzeBytes for TS source with a syntax error %q: SyntaxErrors is empty, want at least one issue", source)
	}
	if len(result.Imports) != 0 || len(result.Findings) != 0 {
		t.Errorf("AnalyzeBytes for TS source with a syntax error %q: Imports/Findings = %+v/%+v, want empty", source, result.Imports, result.Findings)
	}

	if !errors.Is(err, ErrSyntax) {
		t.Errorf("AnalyzeBytes for TS source with a syntax error %q: errors.Is(err, ErrSyntax) = false, want true (err = %v)", source, err)
	}
	var syntaxErr *SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("AnalyzeBytes for TS source with a syntax error %q: errors.As(err, &SyntaxError{}) = false, want true (err = %v)", source, err)
	}
	if len(syntaxErr.Issues) != len(result.SyntaxErrors) {
		t.Errorf("AnalyzeBytes for TS source with a syntax error %q: SyntaxError.Issues length = %d, want %d to match result.SyntaxErrors", source, len(syntaxErr.Issues), len(result.SyntaxErrors))
	}
}
