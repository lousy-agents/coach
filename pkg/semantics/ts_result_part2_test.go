package semantics

import (
	"testing"
)

// AC-R6.1: marshaling the real AnalyzeBytes output for tsGoldenOkSource and
// tsGoldenSyntaxErrorSource must match the checked-in golden files
// byte-for-byte. Unlike goldenOkResult/goldenSyntaxErrorResult (Go's golden
// fixtures, built as Result literals), these Results come from actually
// running the analyzer once, so every byte offset in the golden files is
// generated output, not hand-computed (AC-R6.1's own requirement).
func TestTSResult_MarshalMatchesGoldenFile(t *testing.T) {
	a := mustNewAnalyzer(t)

	tests := []struct {
		name              string
		in                FileInput
		goldenFile        string
		wantErr           bool
		checkRoundTripped func(t *testing.T, r Result)
	}{
		{
			name: "ok",
			in: FileInput{
				Path:     "example.ts",
				Language: LanguageTypeScript,
				Content:  []byte(tsGoldenOkSource),
			},
			goldenFile: "testdata/result_golden_ts_ok.json",
			checkRoundTripped: func(t *testing.T, r Result) {
				body_tsResultPart2Test_34(t, r)
			},
		},
		{
			name: "syntax_errors",
			in: FileInput{
				Path:     "broken.ts",
				Language: LanguageTypeScript,
				Content:  []byte(tsGoldenSyntaxErrorSource),
			},

			goldenFile: "testdata/result_golden_ts_syntax_errors.json",
			wantErr:    true,
			checkRoundTripped: func(t *testing.T, r Result) {
				body_tsResultPart2Test_60(t, r)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_tsResultPart2Test_76(t, a, tt)
		})
	}
}
