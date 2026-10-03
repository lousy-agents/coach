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
				body_resultPart2Test_31(t, r)
			},
		},
		{
			name:       "syntax_errors",
			result:     goldenSyntaxErrorResult(),
			goldenFile: "testdata/result_golden_syntax_errors.json",
			checkRoundTripped: func(t *testing.T, r Result) {
				body_resultPart2Test_48(t, r)
			},
		},
		{
			name:       "react_components",
			result:     goldenReactComponentsResult(),
			goldenFile: "testdata/result_golden_react_components.json",
			checkRoundTripped: func(t *testing.T, r Result) {
				body_resultPart2Test_65(t, r)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_resultPart2Test_100(t, tt)
		})
	}
}
