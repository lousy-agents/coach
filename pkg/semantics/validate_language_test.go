package semantics

import (
	"context"
	"errors"
	"testing"
)

// AC-1.5: any language other than LanguageGo must be rejected with an error
// matching ErrUnsupportedLanguage.
func TestValidate_RejectsUnsupportedLanguage(t *testing.T) {
	result, err := validate(context.Background(), []byte("package main\n"), Language("python"), 0, nil)

	if result != nil {
		t.Errorf("AC-1.5: validate with unsupported language: got result %+v, want nil", result)
	}
	if !errors.Is(err, ErrUnsupportedLanguage) {
		t.Errorf("AC-1.5: validate with unsupported language: got err %v, want errors.Is(err, ErrUnsupportedLanguage)", err)
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
