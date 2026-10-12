package semantics

import (
	"context"
	"encoding/json"
	"testing"
)

// Result.ReactComponents must stay nil (and thus be omitted from JSON via
// omitempty) when AnalyzeBytes finds no React client component candidates:
// Go sources, plain TS/TSX without candidacy, and syntax-error partial
// results.
func TestResult_ReactComponentsEmpty(t *testing.T) {
	a := mustNewAnalyzer(t)

	tests := []reactComponentsOmittedCase{
		{
			name: "go ok",
			input: FileInput{
				Path:     "main.go",
				Language: LanguageGo,
				Content:  []byte("package main\n\nfunc main() {}\n"),
			},
			wantStatus: ParseStatus("ok"),
		},
		{
			name: "typescript ok",
			input: FileInput{
				Path:     "main.ts",
				Language: LanguageTypeScript,
				Content:  []byte("const x: number = 1;\n"),
			},
			wantStatus: ParseStatus("ok"),
		},
		{
			name: "tsx ok",
			input: FileInput{
				Path:     "App.tsx",
				Language: LanguageTSX,
				Content:  []byte("const App = () => <div>hi</div>;\n"),
			},
			wantStatus: ParseStatus("ok"),
		},
		{
			name: "tsx syntax error",
			input: FileInput{
				Path:     "broken.tsx",
				Language: LanguageTSX,
				Content:  []byte("const x = ;"),
			},
			wantStatus: ParseStatus("syntax_errors"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectReactComponentsOmitted(t, a, tt)
		})
	}
}

type reactComponentsOmittedCase struct {
	name       string
	input      FileInput
	wantStatus ParseStatus
}

func expectReactComponentsOmitted(t *testing.T, a *Analyzer, tt reactComponentsOmittedCase) {
	result, _ := a.AnalyzeBytes(context.Background(), tt.input)
	if result == nil {
		t.Fatalf("AnalyzeBytes(%s): got nil result, want non-nil", tt.name)
	}
	if result.ParseStatus != tt.wantStatus {
		t.Fatalf("AnalyzeBytes(%s): ParseStatus = %q, want %q", tt.name, result.ParseStatus, tt.wantStatus)
	}
	if result.ReactComponents != nil {
		t.Errorf("AnalyzeBytes(%s): ReactComponents = %+v, want nil", tt.name, result.ReactComponents)
	}

	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("AnalyzeBytes(%s): marshaling Result must not fail: %v", tt.name, err)
	}
	var asMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &asMap); err != nil {
		t.Fatalf("AnalyzeBytes(%s): Result JSON must unmarshal into a generic map: %v", tt.name, err)
	}
	if _, present := asMap["react_components"]; present {
		t.Errorf("AnalyzeBytes(%s): Result JSON must omit key %q (omitempty), got present with value %s", tt.name, "react_components", asMap["react_components"])
	}
}
