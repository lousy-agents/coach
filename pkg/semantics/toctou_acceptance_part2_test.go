package semantics_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/semantics"
)

// analyzeTSXToctou mirrors analyzeTSToctou but drives the TSX path through
// AnalyzeBytes, so JSX-bearing sources exercise LanguageTSX's registry
// wiring rather than TypeScript's.
func analyzeTSXToctou(analyzer *semantics.Analyzer, source string) *semantics.Result {
	GinkgoHelper()
	result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
		Path:     "example.tsx",
		Language: semantics.LanguageTSX,
		Content:  []byte(source),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result).NotTo(BeNil())
	Expect(result.ParseStatus).To(Equal(semantics.ParseStatus("ok")))
	return result
}
