package semantics

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
)

// goldenOkResult builds the small, hand-legible fixture used by the AC-4.4
// golden test for a clean parse: one import, one finding, non-zero metrics,
// no syntax errors. This is a state AnalyzeBytes can actually return
// (ParseStatus "ok" always carries Imports/Metrics/Findings, never
// SyntaxErrors).
func goldenOkResult() Result {
	return Result{
		Path:        "example.go",
		Language:    LanguageGo,
		ParseStatus: ParseStatus("ok"),
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

// goldenSyntaxErrorResult builds the small, hand-legible fixture used by the
// AC-4.4 golden test for a syntax-error parse: one syntax error, zero-valued
// Metrics, no Imports/Findings. This is the other state AnalyzeBytes can
// actually return (ParseStatus "syntax_errors" always carries SyntaxErrors
// and zero-valued/omitted everything else -- see analyzer.go's HasError
// branch). Kept as a separate fixture from goldenOkResult rather than
// combined into one, since no real Result ever has both SyntaxErrors and
// populated Imports/Metrics/Findings at once.
func goldenSyntaxErrorResult() Result {
	return Result{
		Path:        "broken.go",
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

// AC-4.4: marshaling a Result must match the checked-in golden file
// byte-for-byte, locking field names. Paired with independent field
// assertions on the unmarshaled struct so a future semantic regression
// fails on a readable assertion rather than an opaque byte diff.
//
// Two fixtures/golden files, not one: a real Result never has both
// SyntaxErrors and populated Imports/Metrics/Findings at once (see
// analyzer.go's HasError branch), so combining them into a single
// "fully populated" fixture would lock the JSON shape against a state
// AnalyzeBytes can never actually produce.
func TestResult_MarshalMatchesGoldenFile(t *testing.T) {
	tests := []struct {
		name              string
		result            Result
		goldenFile        string
		checkRoundTripped func(t *testing.T, r Result)
	}{
		{
			name:       "ok",
			result:     goldenOkResult(),
			goldenFile: "testdata/result_golden_ok.json",
			checkRoundTripped: func(t *testing.T, r Result) {
				t.Helper()
				if r.ParseStatus != ParseStatus("ok") {
					t.Errorf("AC-4.4: golden ok Result.ParseStatus: got %q, want %q", r.ParseStatus, "ok")
				}
				if len(r.Imports) != 1 {
					t.Errorf("AC-4.4: golden ok Result.Imports length: got %d, want 1", len(r.Imports))
				}
				if len(r.SyntaxErrors) != 0 {
					t.Errorf("AC-4.4: golden ok Result.SyntaxErrors: got %d, want 0", len(r.SyntaxErrors))
				}
			},
		},
		{
			name:       "syntax_errors",
			result:     goldenSyntaxErrorResult(),
			goldenFile: "testdata/result_golden_syntax_errors.json",
			checkRoundTripped: func(t *testing.T, r Result) {
				t.Helper()
				if r.ParseStatus != ParseStatus("syntax_errors") {
					t.Errorf("AC-4.4: golden syntax_errors Result.ParseStatus: got %q, want %q", r.ParseStatus, "syntax_errors")
				}
				if got, want := r.SyntaxErrors[0].Location.StartByte, uint(10); got != want {
					t.Errorf("AC-4.4: golden syntax_errors Result.SyntaxErrors[0].Location.StartByte: got %d, want %d", got, want)
				}
				if len(r.Imports) != 0 || len(r.Findings) != 0 {
					t.Errorf("AC-4.4: golden syntax_errors Result.Imports/Findings: got %d/%d, want 0/0", len(r.Imports), len(r.Findings))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.MarshalIndent(tt.result, "", "  ")
			if err != nil {
				t.Fatalf("AC-4.4: marshaling the %s Result must not fail: %v", tt.name, err)
			}
			got = append(got, '\n')

			want, err := os.ReadFile(tt.goldenFile)
			if err != nil {
				t.Fatalf("AC-4.4: reading %s must not fail: %v", tt.goldenFile, err)
			}

			if string(got) != string(want) {
				t.Errorf("AC-4.4: %s Result JSON must match golden file byte-for-byte.\ngot:\n%s\nwant:\n%s", tt.name, got, want)
			}

			var roundTripped Result
			if err := json.Unmarshal(want, &roundTripped); err != nil {
				t.Fatalf("AC-4.4: %s golden file must unmarshal back into a Result: %v", tt.name, err)
			}
			tt.checkRoundTripped(t, roundTripped)
		})
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
