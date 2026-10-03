package semantics

import (
	"context"
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
			body_tsResultTest_44(t, tt)
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
