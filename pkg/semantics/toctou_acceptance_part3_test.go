package semantics_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/semantics"
)

// analyzeGoToctou analyzes source as Go and asserts it parsed cleanly,
// mirroring analyzeTSToctou's shape for Go's Story 3 (GitHub issue #179).
func analyzeGoToctou(analyzer *semantics.Analyzer, source string) *semantics.Result {
	GinkgoHelper()
	result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
		Path:     "example.go",
		Language: semantics.LanguageGo,
		Content:  []byte(source),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result).NotTo(BeNil())
	Expect(result.ParseStatus).To(Equal(semantics.ParseStatus("ok")))
	return result
}
