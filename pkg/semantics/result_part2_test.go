package semantics

import (
	"testing"
)

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
	tests := []resultGoldenCase{
		{
			name:       "ok",
			result:     goldenOkResult(),
			goldenFile: "testdata/result_golden_ok.json",
			checkRoundTripped: func(t *testing.T, r Result) {
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
		{
			name:              "react_components",
			result:            goldenReactComponentsResult(),
			goldenFile:        "testdata/result_golden_react_components.json",
			checkRoundTripped: checkGoldenReactComponentsRoundTrip,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectResultMatchesGoldenFile(t, tt)
		})
	}
}
