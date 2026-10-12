package semantics_test

import (
	"context"

	"github.com/lousy-agents/coach/pkg/semantics"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Vitest importOriginal callback pattern in TSX", func() {

	It("parses without a syntax diagnostic", func() {
		analyzer := mustAnalyzer()
		source := []byte(`vi.mock("some-module", async (importOriginal) => {
  const actual = await importOriginal();
  return { ...actual, mockedThing: vi.fn() };
});
`)

		result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
			Path:     "module.test.tsx",
			Language: semantics.LanguageTSX,
			Content:  source,
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(result.ParseStatus).To(Equal(semantics.ParseStatus("ok")))
		Expect(result.SyntaxErrors).To(BeEmpty())
	})

	It("parses the typed generic-call form without a syntax diagnostic (resolves issue #59)", func() {
		analyzer := mustAnalyzer()
		source := []byte(`vi.mock("some-module", async (importOriginal) => {
  const actual = await importOriginal<typeof import("some-module")>();
  return { ...actual, mockedThing: vi.fn() };
});
`)

		result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
			Path:     "module.test.tsx",
			Language: semantics.LanguageTSX,
			Content:  source,
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(result.ParseStatus).To(Equal(semantics.ParseStatus("ok")))
		Expect(result.SyntaxErrors).To(BeEmpty())
	})
})
