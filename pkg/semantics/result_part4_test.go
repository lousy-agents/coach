package semantics

import (
	"encoding/json"

	"testing"
)

// AC-4.1 (omitempty specifics): marshaling a Result with nil SyntaxErrors,
// Imports, and Findings must omit those keys entirely from the JSON output,
// not emit them as null or [].
func TestResult_OmitsEmptyOptionalSlices(t *testing.T) {
	r := Result{
		Path:        "empty.go",
		Language:    LanguageGo,
		ParseStatus: ParseStatus("ok"),
		Metrics:     StructuralMetrics{},
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
			body_resultPart4Test_50(t, tt)
		})
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
			body_resultPart4Test_108(t, tt)
		})
	}
}
