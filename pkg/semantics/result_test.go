package semantics

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
)

// goldenResult builds the small, hand-legible fixture used by the AC-4.4
// golden test: one syntax error, one import, one finding, non-zero metrics.
func goldenResult() Result {
	return Result{
		Path:        "example.go",
		Language:    LanguageGo,
		ParseStatus: ParseStatus("syntax_errors"),
		SyntaxErrors: []SyntaxIssue{
			{
				Kind: "error",
				Location: Location{
					StartByte: 10, EndByte: 11,
					StartRow: 1, StartCol: 0,
					EndRow: 1, EndCol: 1,
				},
			},
		},
		Imports: []ImportFeature{
			{
				Path: "fmt",
				Location: Location{
					StartByte: 20, EndByte: 25,
					StartRow: 2, StartCol: 1,
					EndRow: 2, EndCol: 6,
				},
			},
		},
		Metrics: StructuralMetrics{
			Ifs: 1, Fors: 2, ExprSwitches: 0, TypeSwitches: 0,
			Selects: 0, Functions: 1, Methods: 0, MaxNestingDepth: 2,
		},
		Findings: []Finding{
			{
				Kind: "constructor_func",
				Name: "NewThing",
				Location: Location{
					StartByte: 30, EndByte: 45,
					StartRow: 3, StartCol: 0,
					EndRow: 3, EndCol: 15,
				},
			},
		},
	}
}

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

// AC-4.2: Location must serialize exactly start_byte, end_byte, start_row,
// start_col, end_row, end_col, preserving the 0-based values Tree-sitter
// reports (no offset applied during marshaling).
func TestLocation_JSONHasZeroBasedByteAndRowColFields(t *testing.T) {
	loc := Location{
		StartByte: 0,
		EndByte:   7,
		StartRow:  0,
		StartCol:  0,
		EndRow:    0,
		EndCol:    7,
	}

	raw, err := json.Marshal(loc)
	if err != nil {
		t.Fatalf("AC-4.2: marshaling a Location must not fail: %v", err)
	}

	want := `{"start_byte":0,"end_byte":7,"start_row":0,"start_col":0,"end_row":0,"end_col":7}`
	if string(raw) != want {
		t.Errorf("AC-4.2: Location JSON fields/order/0-based values: got %s, want %s", string(raw), want)
	}
}

// AC-4.3: ParseStatus must serialize as exactly "ok" or "syntax_errors";
// no other values exist in v1.
func TestParseStatus_SerializesAsOkOrSyntaxErrors(t *testing.T) {
	tests := []struct {
		name   string
		status ParseStatus
		want   string
	}{
		{name: "ok status", status: ParseStatus("ok"), want: `"ok"`},
		{name: "syntax_errors status", status: ParseStatus("syntax_errors"), want: `"syntax_errors"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := json.Marshal(tt.status)
			if err != nil {
				t.Fatalf("AC-4.3: marshaling ParseStatus %q must not fail: %v", tt.status, err)
			}
			if string(raw) != tt.want {
				t.Errorf("AC-4.3: ParseStatus %q marshaled: got %s, want %s", tt.status, string(raw), tt.want)
			}
		})
	}
}

// AC-4.4: marshaling a fully populated Result must match the checked-in
// golden file byte-for-byte, locking field names. Paired with independent
// field assertions on the unmarshaled struct so a future semantic
// regression fails on a readable assertion rather than an opaque byte diff.
func TestResult_MarshalMatchesGoldenFile(t *testing.T) {
	r := goldenResult()

	got, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatalf("AC-4.4: marshaling a fully populated Result must not fail: %v", err)
	}
	got = append(got, '\n')

	want, err := os.ReadFile("testdata/result_golden.json")
	if err != nil {
		t.Fatalf("AC-4.4: reading testdata/result_golden.json must not fail: %v", err)
	}

	if string(got) != string(want) {
		t.Errorf("AC-4.4: Result JSON must match golden file byte-for-byte.\ngot:\n%s\nwant:\n%s", got, want)
	}

	var roundTripped Result
	if err := json.Unmarshal(want, &roundTripped); err != nil {
		t.Fatalf("AC-4.4: golden file must unmarshal back into a Result: %v", err)
	}

	if roundTripped.ParseStatus != ParseStatus("syntax_errors") {
		t.Errorf("AC-4.4: golden Result.ParseStatus: got %q, want %q", roundTripped.ParseStatus, "syntax_errors")
	}
	if len(roundTripped.Imports) != 1 {
		t.Errorf("AC-4.4: golden Result.Imports length: got %d, want 1", len(roundTripped.Imports))
	}
	if got, want := roundTripped.SyntaxErrors[0].Location.StartByte, uint(10); got != want {
		t.Errorf("AC-4.4: golden Result.SyntaxErrors[0].Location.StartByte: got %d, want %d", got, want)
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
	tests := []struct {
		name   string
		issues []SyntaxIssue
		want   string
	}{
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
			err := &SyntaxError{Issues: tt.issues}
			if got := err.Error(); got != tt.want {
				t.Errorf("AC-2.3: SyntaxError.Error() for %d issue(s): got %q, want %q", len(tt.issues), got, tt.want)
			}
		})
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

	seen := make(map[string]string) // message -> sentinel name
	for name, err := range sentinels {
		msg := err.Error()
		if other, ok := seen[msg]; ok {
			t.Errorf("sentinels %s and %s must have distinct messages, both are %q", name, other, msg)
		}
		seen[msg] = name
	}
}

// AC-4.1 (omitempty specifics): marshaling a Result with nil SyntaxErrors,
// Imports, and Findings must omit those keys entirely from the JSON output,
// not emit them as null or [].
func TestResult_OmitsEmptyOptionalSlices(t *testing.T) {
	r := Result{
		Path:        "empty.go",
		Language:    LanguageGo,
		ParseStatus: ParseStatus("ok"),
		Metrics:     StructuralMetrics{},
		// SyntaxErrors, Imports, Findings left nil.
	}

	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshaling a Result with nil optional slices must not fail: %v", err)
	}

	var asMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &asMap); err != nil {
		t.Fatalf("Result JSON must unmarshal into a generic map: %v", err)
	}

	for _, key := range []string{"syntax_errors", "imports", "findings"} {
		if raw, present := asMap[key]; present {
			t.Errorf("Result JSON must omit key %q for nil slice (omitempty), got present with value %s", key, raw)
		}
	}
}
