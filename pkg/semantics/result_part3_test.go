package semantics

import (
	"encoding/json"
	"errors"

	"testing"
)

// AC-4.1: the Result type (and its nested types) must carry explicit json
// struct tags in snake_case.
func TestResult_JSONUsesSnakeCaseTags(t *testing.T) {
	r := Result{
		Path:        "main.go",
		Language:    LanguageGo,
		ParseStatus: ParseStatus("ok"),
		Metrics:     StructuralMetrics{Ifs: 1},
	}

	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("AC-4.1: marshaling a minimal Result must not fail: %v", err)
	}

	var asMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &asMap); err != nil {
		t.Fatalf("AC-4.1: Result JSON must unmarshal into a generic map: %v", err)
	}

	wantKeys := []string{"path", "language", "parse_status", "metrics"}
	for _, key := range wantKeys {
		if _, ok := asMap[key]; !ok {
			t.Errorf("AC-4.1: Result JSON missing expected snake_case key %q; got keys: %v", key, asMap)
		}
	}

	badKeys := []string{"Path", "Language", "ParseStatus", "Metrics"}
	for _, key := range badKeys {
		if _, ok := asMap[key]; ok {
			t.Errorf("AC-4.1: Result JSON must not use PascalCase key %q; got keys: %v", key, asMap)
		}
	}
}

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
