package semantics_test

import (
	"context"

	"github.com/lousy-agents/coach/pkg/semantics"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func toctouFindings(findings []semantics.Finding) []semantics.Finding {
	var out []semantics.Finding
	for _, f := range findings {
		if f.Kind == "toctou_check_then_act" {
			out = append(out, f)
		}
	}
	return out
}

// analyzeTSToctou analyzes source as TypeScript and asserts it parsed
// cleanly, mirroring analyzeTSCC's shape for a different worked-example
// family (GitHub issue #177, Story 1).
func analyzeTSToctou(analyzer *semantics.Analyzer, source string) semantics.Result {
	GinkgoHelper()
	result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
		Path:     "example.ts",
		Language: semantics.LanguageTypeScript,
		Content:  []byte(source),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result).NotTo(BeNil())
	Expect(result.ParseStatus).To(Equal(semantics.ParseStatus("ok")))
	return *result
}

// analyzeTSXToctou mirrors analyzeTSToctou but drives the TSX path through
// AnalyzeBytes, so JSX-bearing sources exercise LanguageTSX's registry
// wiring rather than TypeScript's.
func analyzeTSXToctou(analyzer *semantics.Analyzer, source string) semantics.Result {
	GinkgoHelper()
	result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
		Path:     "example.tsx",
		Language: semantics.LanguageTSX,
		Content:  []byte(source),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result).NotTo(BeNil())
	Expect(result.ParseStatus).To(Equal(semantics.ParseStatus("ok")))
	return *result
}

// analyzeGoToctou analyzes source as Go and asserts it parsed cleanly,
// mirroring analyzeTSToctou's shape for Go's Story 3 (GitHub issue #179).
func analyzeGoToctou(analyzer *semantics.Analyzer, source string) semantics.Result {
	GinkgoHelper()
	result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
		Path:     "example.go",
		Language: semantics.LanguageGo,
		Content:  []byte(source),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result).NotTo(BeNil())
	Expect(result.ParseStatus).To(Equal(semantics.ParseStatus("ok")))
	return *result
}
