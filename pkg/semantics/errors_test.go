package semantics

import (
	"errors"
	"fmt"
	"testing"
)

// Sentinel existence (supports AC-1.4, AC-1.5, AC-1.6, AC-1.7, AC-2.3,
// AC-6.2): all six sentinels must exist, be non-nil, and be pairwise
// distinct by message. Full behavioral coverage of these sentinels (i.e.
// that AnalyzeBytes actually returns them) belongs to later tasks; this
// task only guarantees the sentinels exist since errors.go is not revisited.
func TestSentinelErrors_AreNonNilAndPairwiseDistinct(t *testing.T) {
	sentinels := map[string]error{
		"ErrEmptyContent":        ErrEmptyContent,
		"ErrUnsupportedLanguage": ErrUnsupportedLanguage,
		"ErrFileTooLarge":        ErrFileTooLarge,
		"ErrBinaryContent":       ErrBinaryContent,
		"ErrSyntax":              ErrSyntax,
		"ErrParseFailure":        ErrParseFailure,
	}

	for name, err := range sentinels {
		if err == nil {
			t.Errorf("sentinel %s must be non-nil", name)
		}
	}

	seen := make(map[string]string)
	for name, err := range sentinels {
		msg := err.Error()
		if other, ok := seen[msg]; ok {
			t.Errorf("sentinels %s and %s must have distinct messages, both are %q", name, other, msg)
		}
		seen[msg] = name
	}
}

// AC-2.3: errors.Is(err, ErrSyntax) must recognize a *SyntaxError via its Is
// method, regardless of the Issues it carries.
func TestSyntaxError_IsMatchesErrSyntax(t *testing.T) {
	err := &SyntaxError{Issues: []SyntaxIssue{{Kind: "error"}}}

	if !errors.Is(err, ErrSyntax) {
		t.Errorf("AC-2.3: errors.Is(%v, ErrSyntax) = false, want true", err)
	}
}

// AC-2.3: errors.As must extract a *SyntaxError from a wrapping error chain,
// and its Issues must match what was originally constructed.
func TestSyntaxError_AsExtractsIssues(t *testing.T) {
	issues := []SyntaxIssue{
		{Kind: "error", Location: Location{StartByte: 1, EndByte: 2}},
		{Kind: "missing", Location: Location{StartByte: 3, EndByte: 3}},
	}
	original := &SyntaxError{Issues: issues}
	wrapped := fmt.Errorf("analyzing file: %w", original)

	var got *SyntaxError
	if !errors.As(wrapped, &got) {
		t.Fatalf("AC-2.3: errors.As(%v, &SyntaxError{}) = false, want true", wrapped)
	}

	if len(got.Issues) != len(issues) {
		t.Fatalf("AC-2.3: extracted SyntaxError.Issues length: got %d, want %d", len(got.Issues), len(issues))
	}
	for i, want := range issues {
		if got.Issues[i] != want {
			t.Errorf("AC-2.3: extracted SyntaxError.Issues[%d]: got %+v, want %+v", i, got.Issues[i], want)
		}
	}
}

// AC-2.3: SyntaxError.Error() must produce a string summarizing the number
// of issues, so log lines and top-level error messages are useful without
// needing to inspect Issues directly.
func TestSyntaxError_ErrorSummarizesIssueCount(t *testing.T) {
	tests := []syntaxErrorMessageCase{
		{
			name:   "one issue",
			issues: []SyntaxIssue{{Kind: "error"}},
			want:   "semantics: 1 syntax issue(s)",
		},
		{
			name:   "three issues",
			issues: []SyntaxIssue{{Kind: "error"}, {Kind: "missing"}, {Kind: "error"}},
			want:   "semantics: 3 syntax issue(s)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectSyntaxErrorMessage(t, tt)
		})
	}
}

type syntaxErrorMessageCase struct {
	name   string
	issues []SyntaxIssue
	want   string
}

func expectSyntaxErrorMessage(t *testing.T, tt syntaxErrorMessageCase) {
	err := &SyntaxError{Issues: tt.issues}
	if got := err.Error(); got != tt.want {
		t.Errorf("AC-2.3: SyntaxError.Error() for %d issue(s): got %q, want %q", len(tt.issues), got, tt.want)
	}
}
