package codesignal

import (
	"context"
	"os"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

type hiddenInputMutationFixture struct {
	name           string
	srcPath        string
	resultPath     string
	lang           semantics.Language
	wantConfidence Confidence
}

func mustAnalyzeFixture(t *testing.T, srcPath, resultPath string, lang semantics.Language) *semantics.Result {
	t.Helper()

	content, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", srcPath, err)
	}

	analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
	if err != nil {
		t.Fatalf("semantics.NewAnalyzer: %v", err)
	}

	result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
		Path:     resultPath,
		Language: lang,
		Content:  content,
	})
	if err != nil {
		t.Fatalf("AnalyzeBytes(%s): %v", srcPath, err)
	}

	return result
}

func firstMutatesInputFinding(result *semantics.Result) (semantics.Finding, bool) {
	for _, finding := range result.Findings {
		if finding.Kind == "mutates_input" {
			return finding, true
		}
	}
	return semantics.Finding{}, false
}
