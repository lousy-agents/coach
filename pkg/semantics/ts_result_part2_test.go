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

	tests := []tsResultGoldenCase{
		{
			name: "ok",
			in: FileInput{
				Path:     "example.ts",
				Language: LanguageTypeScript,
				Content:  []byte(tsGoldenOkSource),
			},
			goldenFile: "testdata/result_golden_ts_ok.json",
			checkRoundTripped: func(t *testing.T, r Result) {
				if r.ParseStatus != ParseStatus("ok") {
					t.Errorf("AC-R6.1: golden TS ok Result.ParseStatus: got %q, want %q", r.ParseStatus, "ok")
				}
				if len(r.Imports) != 1 || r.Imports[0].Path != "./http" {
					t.Errorf("AC-R6.1: golden TS ok Result.Imports: got %+v, want one import with Path %q", r.Imports, "./http")
				}
				if len(r.Findings) != 1 || r.Findings[0].Kind != "tight_coupling" || r.Findings[0].Name != "HttpClient" {
					t.Errorf("AC-R6.1: golden TS ok Result.Findings: got %+v, want one tight_coupling finding named %q", r.Findings, "HttpClient")
				}
				if len(r.SyntaxErrors) != 0 {
					t.Errorf("AC-R6.1: golden TS ok Result.SyntaxErrors: got %d, want 0", len(r.SyntaxErrors))
				}
			},
		},
		{
			name: "syntax_errors",
			in: FileInput{
				Path:     "broken.ts",
				Language: LanguageTypeScript,
				Content:  []byte(tsGoldenSyntaxErrorSource),
			},

			goldenFile:        "testdata/result_golden_ts_syntax_errors.json",
			wantErr:           true,
			checkRoundTripped: checkTSSyntaxErrorGoldenRoundTrip,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectTSResultMatchesGoldenFile(t, a, tt)
		})
	}
}
