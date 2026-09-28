package semantics

import (
	"context"

	"errors"

	"testing"
)

func body_analyzerPart6Test_48(t *testing.T, a *Analyzer, tt struct {
	name string
	lang Language
	src  string
}) {
	result, err := a.AnalyzeBytes(context.Background(), FileInput{
		Path:     "broken",
		Language: tt.lang,
		Content:  []byte(tt.src),
	})
	if result == nil {
		t.Fatalf("AnalyzeBytes(%q): got nil result, want a partial *Result", tt.src)
	}
	if result.ParseStatus != ParseStatus("syntax_errors") {
		t.Errorf("AnalyzeBytes(%q): ParseStatus = %q, want %q", tt.src, result.ParseStatus, "syntax_errors")
	}
	if len(result.SyntaxErrors) == 0 {
		t.Errorf("AnalyzeBytes(%q): SyntaxErrors is empty, want at least one issue", tt.src)
	}
	if !errors.Is(err, ErrSyntax) {
		t.Errorf("AnalyzeBytes(%q): errors.Is(err, ErrSyntax) = false, want true (err = %v)", tt.src, err)
	}
}

func body_analyzerPart6Test_98(t *testing.T, a *Analyzer, tt struct {
	name string
	lang Language
	src  string
}) {
	result, err := a.AnalyzeBytes(context.Background(), FileInput{
		Path:     "ok",
		Language: tt.lang,
		Content:  []byte(tt.src),
	})
	if err != nil {
		t.Fatalf("AnalyzeBytes(%q): got err %v, want nil", tt.src, err)
	}
	if result == nil {
		t.Fatalf("AnalyzeBytes(%q): got nil result, want a valid *Result", tt.src)
	}
	if result.ParseStatus != ParseStatus("ok") {
		t.Errorf("AnalyzeBytes(%q): ParseStatus = %q, want %q (SyntaxErrors = %+v)", tt.src, result.ParseStatus, "ok", result.SyntaxErrors)
	}
	if len(result.SyntaxErrors) != 0 {
		t.Errorf("AnalyzeBytes(%q): SyntaxErrors = %+v, want empty", tt.src, result.SyntaxErrors)
	}
}
