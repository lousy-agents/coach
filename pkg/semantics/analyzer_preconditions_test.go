package semantics

import (
	"context"
	"errors"
	"testing"
)

// Regression guard raised by review: AC-1.4 through AC-1.7 were previously
// exercised only against the unexported validate function (parser_test.go),
// never through the public AnalyzeBytes facade that wires validate in as
// its first pipeline step. A regression that stopped calling validate, or
// called it with the wrong arguments, would not have been caught by any
// test. This drives each precondition through AnalyzeBytes itself.
func TestAnalyzeBytes_RejectsInvalidInputThroughPublicFacade(t *testing.T) {
	tests := []rejectedInputCase{
		{
			name:    "AC-1.4: empty content",
			in:      FileInput{Language: LanguageGo, Content: []byte{}},
			wantErr: ErrEmptyContent,
		},
		{
			name:    "AC-1.5: unsupported language",
			in:      FileInput{Language: "python", Content: []byte("package main\n")},
			wantErr: ErrUnsupportedLanguage,
		},
		{
			name:    "AC-1.7: content containing a NUL byte",
			in:      FileInput{Language: LanguageGo, Content: []byte("package main\x00\n")},
			wantErr: ErrBinaryContent,
		},
	}

	a := mustNewAnalyzer(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectAnalyzeBytesRejects(t, a, tt)
		})
	}

	t.Run("AC-1.6: content over MaxFileBytes", func(t *testing.T) {
		small, err := NewAnalyzer(AnalyzerOptions{MaxFileBytes: 4})
		if err != nil {
			t.Fatalf("NewAnalyzer(MaxFileBytes: 4): got err %v, want nil", err)
		}

		in := FileInput{Language: LanguageGo, Content: []byte("package main\n")}
		result, err := small.AnalyzeBytes(context.Background(), in)

		if result != nil {
			t.Errorf("AnalyzeBytes(%+v) with MaxFileBytes=4: got non-nil result %+v, want nil", in, result)
		}
		if !errors.Is(err, ErrFileTooLarge) {
			t.Errorf("AnalyzeBytes(%+v) with MaxFileBytes=4: got err %v, want errors.Is(err, ErrFileTooLarge) to hold", in, err)
		}
	})
}

type rejectedInputCase struct {
	name    string
	in      FileInput
	wantErr error
}

func expectAnalyzeBytesRejects(t *testing.T, a *Analyzer, tt rejectedInputCase) {
	result, err := a.AnalyzeBytes(context.Background(), tt.in)

	if result != nil {
		t.Errorf("AnalyzeBytes(%+v): got non-nil result %+v, want nil", tt.in, result)
	}
	if !errors.Is(err, tt.wantErr) {
		t.Errorf("AnalyzeBytes(%+v): got err %v, want errors.Is(err, %v) to hold", tt.in, err, tt.wantErr)
	}
}

// AC-R2.4: a Language not in languageRegistry (e.g. "javascript", which is
// explicitly out of scope) must return ErrUnsupportedLanguage and a nil
// *Result, unchanged from the pre-existing contract.
func TestAnalyzeBytes_RejectsJavaScriptAsUnsupported(t *testing.T) {
	a := mustNewAnalyzer(t)

	result, err := a.AnalyzeBytes(context.Background(), FileInput{
		Language: "javascript",
		Content:  []byte(`const x = 1;`),
	})

	if result != nil {
		t.Errorf("AnalyzeBytes with Language \"javascript\": got non-nil result %+v, want nil", result)
	}
	if !errors.Is(err, ErrUnsupportedLanguage) {
		t.Errorf("AnalyzeBytes with Language \"javascript\": got err %v, want errors.Is(err, ErrUnsupportedLanguage)", err)
	}
}

// AC-R2.6: the language-agnostic preconditions in validate (empty content,
// binary content, oversized content) apply identically to TS, confirming
// validate needs no per-language change to support the new grammar.
func TestAnalyzeBytes_TSPreconditionsMatchGoPreconditions(t *testing.T) {
	a := mustNewAnalyzer(t)

	t.Run("empty content", func(t *testing.T) {
		result, err := a.AnalyzeBytes(context.Background(), FileInput{Language: LanguageTypeScript, Content: []byte{}})
		thenResultIsNil(t, result, "AnalyzeBytes for TS with empty content")
		thenErrorIs(t, err, ErrEmptyContent, "AnalyzeBytes for TS with empty content")
	})

	t.Run("binary content", func(t *testing.T) {
		result, err := a.AnalyzeBytes(context.Background(), FileInput{Language: LanguageTypeScript, Content: []byte("const x\x00 = 1;")})
		thenResultIsNil(t, result, "AnalyzeBytes for TS with a NUL byte")
		thenErrorIs(t, err, ErrBinaryContent, "AnalyzeBytes for TS with a NUL byte")
	})

	t.Run("content over MaxFileBytes", func(t *testing.T) {
		small, err := NewAnalyzer(AnalyzerOptions{MaxFileBytes: 4})
		if err != nil {
			t.Fatalf("NewAnalyzer(MaxFileBytes: 4): got err %v, want nil", err)
		}
		result, err := small.AnalyzeBytes(context.Background(), FileInput{Language: LanguageTypeScript, Content: []byte("const x = 1;")})
		thenResultIsNil(t, result, "AnalyzeBytes for TS over MaxFileBytes")
		thenErrorIs(t, err, ErrFileTooLarge, "AnalyzeBytes for TS over MaxFileBytes")
	})
}
