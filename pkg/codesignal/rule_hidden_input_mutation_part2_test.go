package codesignal

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestHiddenInputMutation_EndToEndPerLanguage(t *testing.T) {
	tests := []struct {
		name           string
		srcPath        string
		resultPath     string
		lang           semantics.Language
		wantConfidence Confidence
	}{
		{
			name:           "go",
			srcPath:        "../../internal/jsbridge/testdata/parity/go_mutates_input.src",
			resultPath:     "example/mutate.go",
			lang:           semantics.LanguageGo,
			wantConfidence: "medium",
		},
		{
			name:           "typescript",
			srcPath:        "../../internal/jsbridge/testdata/parity/ts_mutates_input.src",
			resultPath:     "example/mutate.ts",
			lang:           semantics.LanguageTypeScript,
			wantConfidence: "medium",
		},
		{
			name:           "tsx",
			srcPath:        "../../internal/jsbridge/testdata/parity/tsx_mutates_input.src",
			resultPath:     "example/mutate.tsx",
			lang:           semantics.LanguageTSX,
			wantConfidence: "medium",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_ruleHiddenInputMutationPart2Test_43(t, tt)
		})
	}
}
