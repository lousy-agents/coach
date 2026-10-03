package semantics

import (
	"context"

	"testing"
)

func body_analyzerPart8Test_contentOverMaxFileBytes_51(t *testing.T) {
	small, err := NewAnalyzer(AnalyzerOptions{MaxFileBytes: 4})
	if err != nil {
		t.Fatalf("NewAnalyzer(MaxFileBytes: 4): got err %v, want nil", err)
	}
	result, err := small.AnalyzeBytes(context.Background(), FileInput{Language: LanguageTypeScript, Content: []byte("const x = 1;")})
	thenResultIsNil(t, result, "AnalyzeBytes for TS over MaxFileBytes")
	thenErrorIs(t, err, ErrFileTooLarge, "AnalyzeBytes for TS over MaxFileBytes")
}
