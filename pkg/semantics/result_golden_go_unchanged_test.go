package semantics

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

// AC-R6.2: adding TS support must not change either existing Go golden
// file. This is a regression guard that fails loudly (rather than via
// `git diff`, which CI checks separately) if a future change to shared
// pipeline code alters Go's frozen output.
func TestGoGoldenFiles_UnchangedByTSSupport(t *testing.T) {
	a := mustNewAnalyzer(t)

	tests := []goGoldenFileCase{
		{name: "ok", result: goldenOkResult(), goldenFile: "testdata/result_golden_ok.json"},
		{name: "syntax_errors", result: goldenSyntaxErrorResult(), goldenFile: "testdata/result_golden_syntax_errors.json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectGoGoldenFileUnchanged(t, tt)
		})
	}

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

type goGoldenFileCase struct {
	name       string
	result     Result
	goldenFile string
}

func expectGoGoldenFileUnchanged(t *testing.T, tt goGoldenFileCase) {
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
}
