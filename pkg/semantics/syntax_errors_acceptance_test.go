package semantics_test

import (
	"context"
	"errors"

	"github.com/lousy-agents/coach/pkg/semantics"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("syntax error reporting", func() {
	var analyzer *semantics.Analyzer

	BeforeEach(func() {
		analyzer = mustAnalyzer()
	})

	Context("when Go source contains a syntax error", func() {
		It("returns a partial result with parse_status \"syntax_errors\" and zero-valued features (AC-2.1, AC-2.2)", func() {
			source := []byte("package main\nfunc {")

			result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
				Path:     "broken.go",
				Language: semantics.LanguageGo,
				Content:  source,
			})

			Expect(result).NotTo(BeNil())
			Expect(result.ParseStatus).To(Equal(semantics.ParseStatus("syntax_errors")))
			Expect(result.SyntaxErrors).NotTo(BeEmpty())
			Expect(result.Imports).To(BeEmpty())
			Expect(result.Findings).To(BeEmpty())
			Expect(result.CognitiveComplexity).To(BeEmpty())
			Expect(result.Metrics).To(Equal(semantics.StructuralMetrics{}))
			Expect(err).To(HaveOccurred())
		})

		It("returns an error matching ErrSyntax that unwraps to a *SyntaxError consistent with the result (AC-2.3)", func() {
			source := []byte("package main\nfunc {")

			result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
				Path:     "broken.go",
				Language: semantics.LanguageGo,
				Content:  source,
			})

			Expect(errors.Is(err, semantics.ErrSyntax)).To(BeTrue())

			var syntaxErr *semantics.SyntaxError
			Expect(errors.As(err, &syntaxErr)).To(BeTrue())
			Expect(syntaxErr.Issues).To(Equal(result.SyntaxErrors))
		})
	})

	Context("when the grammar's error recovery produces a zero-width MISSING node", func() {
		It("reports a location where start_byte equals end_byte, without error (AC-2.5)", func() {
			source := []byte("package main\nfunc f() {\n\tg(1, 2\n}\n")

			result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
				Path:     "broken.go",
				Language: semantics.LanguageGo,
				Content:  source,
			})

			Expect(err).To(HaveOccurred())
			Expect(result).NotTo(BeNil())

			var missing *semantics.SyntaxIssue
			for i := range result.SyntaxErrors {
				if result.SyntaxErrors[i].Kind == "missing" {
					missing = &result.SyntaxErrors[i]
					break
				}
			}
			Expect(missing).NotTo(BeNil(), "expected at least one \"missing\" syntax issue, got %+v", result.SyntaxErrors)
			Expect(missing.Location.StartByte).To(Equal(missing.Location.EndByte))
		})
	})
})
