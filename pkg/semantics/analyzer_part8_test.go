package semantics

import (
	"context"

	"reflect"

	"testing"
)

// Constructor validation: NewAnalyzer must reject a negative MaxFileBytes,
// since a negative size limit is nonsensical (per the frozen API doc: "0 =
// default 2 MiB; negative = NewAnalyzer returns an error").
func TestNewAnalyzer_RejectsNegativeMaxFileBytes(t *testing.T) {
	_, err := NewAnalyzer(AnalyzerOptions{MaxFileBytes: -1})

	if err == nil {
		t.Fatalf("NewAnalyzer with MaxFileBytes = -1: got nil error, want non-nil")
	}
}

// AC-6.3: Analyzer must hold no C-backed resources between calls (Parser,
// Tree, Query, QueryCursor are all created fresh inside AnalyzeBytes), so it
// must expose no Close method for callers to forget to call.
func TestAnalyzer_HasNoExportedCloseMethod(t *testing.T) {
	_, ok := reflect.TypeOf(&Analyzer{}).MethodByName("Close")

	if ok {
		t.Errorf("AC-6.3: (*Analyzer).Close must not exist, want ok == false, got ok == true")
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
