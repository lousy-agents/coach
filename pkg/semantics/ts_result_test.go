package semantics

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

// tsGoldenOkSource is the small, hand-legible fixture used by the AC-R6.1
// golden test for a clean TS parse: one import, one metric-bearing
// construct, and one tight_coupling finding.
const tsGoldenOkSource = `import { HttpClient } from "./http";

class Greeter {
	constructor() {
		this.client = new HttpClient();
	}
}
`

// tsGoldenSyntaxErrorSource is the fixture used by the AC-R6.1 golden test
// for a TS syntax-error parse.
const tsGoldenSyntaxErrorSource = `const x = ;
`

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
				t.Helper()
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
			goldenFile: "testdata/result_golden_ts_syntax_errors.json",
			wantErr:    true,
			checkRoundTripped: func(t *testing.T, r Result) {
				t.Helper()
				if r.ParseStatus != ParseStatus("syntax_errors") {
					t.Errorf("AC-R6.1: golden TS syntax_errors Result.ParseStatus: got %q, want %q", r.ParseStatus, "syntax_errors")
				}
				if len(r.SyntaxErrors) == 0 {
					t.Errorf("AC-R6.1: golden TS syntax_errors Result.SyntaxErrors: got empty, want at least one issue")
				}
				if len(r.Imports) != 0 || len(r.Findings) != 0 {
					t.Errorf("AC-R6.1: golden TS syntax_errors Result.Imports/Findings: got %d/%d, want 0/0", len(r.Imports), len(r.Findings))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := a.AnalyzeBytes(context.Background(), tt.in)
			if tt.wantErr && err == nil {
				t.Fatalf("AnalyzeBytes(%+v): got nil err, want non-nil", tt.in)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("AnalyzeBytes(%+v): got err %v, want nil", tt.in, err)
			}
			if result == nil {
				t.Fatalf("AnalyzeBytes(%+v): got nil result, want non-nil", tt.in)
			}

			got, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				t.Fatalf("AC-R6.1: marshaling the %s Result must not fail: %v", tt.name, err)
			}
			got = append(got, '\n')

			want, err := os.ReadFile(tt.goldenFile)
			if err != nil {
				t.Fatalf("AC-R6.1: reading %s must not fail: %v", tt.goldenFile, err)
			}

			if string(got) != string(want) {
				t.Errorf("AC-R6.1: %s Result JSON must match golden file byte-for-byte.\ngot:\n%s\nwant:\n%s", tt.name, got, want)
			}

			var roundTripped Result
			if err := json.Unmarshal(want, &roundTripped); err != nil {
				t.Fatalf("AC-R6.1: %s golden file must unmarshal back into a Result: %v", tt.name, err)
			}
			tt.checkRoundTripped(t, roundTripped)
		})
	}
}

// AC-R6.2: adding TS support must not change either existing Go golden
// file. This is a regression guard that fails loudly (rather than via
// `git diff`, which CI checks separately) if a future change to shared
// pipeline code alters Go's frozen output.
func TestGoGoldenFiles_UnchangedByTSSupport(t *testing.T) {
	a := mustNewAnalyzer(t)

	tests := []struct {
		name       string
		result     Result
		goldenFile string
	}{
		{name: "ok", result: goldenOkResult(), goldenFile: "testdata/result_golden_ok.json"},
		{name: "syntax_errors", result: goldenSyntaxErrorResult(), goldenFile: "testdata/result_golden_syntax_errors.json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.MarshalIndent(tt.result, "", "  ")
			if err != nil {
				t.Fatalf("marshaling the Go %s fixture must not fail: %v", tt.name, err)
			}
			got = append(got, '\n')

			want, err := os.ReadFile(tt.goldenFile)
			if err != nil {
				t.Fatalf("reading %s must not fail: %v", tt.goldenFile, err)
			}
			if string(got) != string(want) {
				t.Errorf("AC-R6.2: %s must remain byte-identical after adding TS support.\ngot:\n%s\nwant:\n%s", tt.goldenFile, got, want)
			}
		})
	}

	// Also confirm Go analysis itself (not just the hand-built fixtures
	// above) is unaffected end-to-end by the new TS registry entries.
	result, err := a.AnalyzeBytes(context.Background(), FileInput{
		Path:     "main.go",
		Language: LanguageGo,
		Content:  []byte("package main\nfunc main() {}\n"),
	})
	if err != nil {
		t.Fatalf("AnalyzeBytes for Go source after adding TS support: got err %v, want nil", err)
	}
	if result.Language != LanguageGo || result.ParseStatus != ParseStatus("ok") {
		t.Errorf("AnalyzeBytes for Go source after adding TS support: got %+v, want Language=go ParseStatus=ok", result)
	}
}
