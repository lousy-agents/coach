package semantics_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"

	"github.com/lousy-agents/coach/pkg/semantics"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("the semantic analyzer", func() {
	var analyzer *semantics.Analyzer

	BeforeEach(func() {
		analyzer = mustAnalyzer()
	})

	Context("when an external consumer supplies valid Go source bytes", func() {
		It("returns deterministic structural facts with no error (AC-1.2)", func() {
			source := []byte("package main\n\nfunc Hello() string {\n\treturn \"world\"\n}\n")

			result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
				Path:     "hello.go",
				Language: semantics.LanguageGo,
				Content:  source,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.ParseStatus).To(Equal(semantics.ParseStatus("ok")))
		})

		It("returns byte-identical JSON for repeated analysis of the same input (AC-1.3)", func() {
			in := semantics.FileInput{
				Path:     "main.go",
				Language: semantics.LanguageGo,
				Content: []byte(`package main

import "fmt"

func NewFoo() *int {
	if true {
		fmt.Println("hi")
	}
	return nil
}
`),
			}

			first, err := analyzer.AnalyzeBytes(context.Background(), in)
			Expect(err).NotTo(HaveOccurred())
			second, err := analyzer.AnalyzeBytes(context.Background(), in)
			Expect(err).NotTo(HaveOccurred())

			firstJSON, err := json.Marshal(first)
			Expect(err).NotTo(HaveOccurred())
			secondJSON, err := json.Marshal(second)
			Expect(err).NotTo(HaveOccurred())

			Expect(secondJSON).To(Equal(firstJSON))
		})

		It("orders imports and findings by document position (AC-1.10)", func() {
			expectImportsAndFindingsInDocumentOrder(analyzer)
		})

		It("orders syntax errors by document position (AC-1.10)", func() {
			source := []byte("package main\nfunc f() {\nfunc g() {\n")

			result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
				Path:     "main.go",
				Language: semantics.LanguageGo,
				Content:  source,
			})
			Expect(errors.Is(err, semantics.ErrSyntax)).To(BeTrue())

			Expect(len(result.SyntaxErrors)).To(BeNumerically(">=", 2))
			for i := 1; i < len(result.SyntaxErrors); i++ {
				Expect(result.SyntaxErrors[i].Location.StartByte).To(BeNumerically(">=", result.SyntaxErrors[i-1].Location.StartByte))
			}
		})
	})

	Context("when the input violates a documented precondition", func() {
		DescribeTable("rejects the input with the documented sentinel error",
			func(build func() (*semantics.Analyzer, semantics.FileInput), wantErr error) {
				a, in := build()
				result, err := a.AnalyzeBytes(context.Background(), in)

				Expect(result).To(BeNil())
				Expect(errors.Is(err, wantErr)).To(BeTrue(), "got err %v, want errors.Is(err, %v)", err, wantErr)
			},
			Entry("empty content (AC-1.4)", func() (*semantics.Analyzer, semantics.FileInput) {
				return mustAnalyzer(), semantics.FileInput{Language: semantics.LanguageGo, Content: []byte{}}
			}, semantics.ErrEmptyContent),
			Entry("unsupported language (AC-1.5)", func() (*semantics.Analyzer, semantics.FileInput) {
				return mustAnalyzer(), semantics.FileInput{Language: "python", Content: []byte("package main\n")}
			}, semantics.ErrUnsupportedLanguage),
			Entry("content over MaxFileBytes (AC-1.6)", func() (*semantics.Analyzer, semantics.FileInput) {
				small, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{MaxFileBytes: 4})
				Expect(err).NotTo(HaveOccurred())
				return small, semantics.FileInput{Language: semantics.LanguageGo, Content: []byte("package main\n")}
			}, semantics.ErrFileTooLarge),
			Entry("content containing a NUL byte (AC-1.7)", func() (*semantics.Analyzer, semantics.FileInput) {
				return mustAnalyzer(), semantics.FileInput{Language: semantics.LanguageGo, Content: []byte("package main\x00\n")}
			}, semantics.ErrBinaryContent),
		)

		It("returns ctx.Err() for an already-cancelled context (AC-1.8)", func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			result, err := analyzer.AnalyzeBytes(ctx, semantics.FileInput{
				Language: semantics.LanguageGo,
				Content:  []byte("package main\nfunc main() {}\n"),
			})

			Expect(result).To(BeNil())
			Expect(errors.Is(err, context.Canceled)).To(BeTrue())
		})
	})

	Context("when one Analyzer is used by multiple goroutines at once", func() {
		It("is safe for concurrent callers (AC-1.9; run under go test -race)", func() {
			source := []byte("package main\nfunc main() {}\n")
			const goroutines = 8

			results := make([]*semantics.Result, goroutines)
			errs := make([]error, goroutines)
			var wg sync.WaitGroup
			wg.Add(goroutines)
			for i := 0; i < goroutines; i++ {
				go func(i int) {
					defer wg.Done()
					results[i], errs[i] = analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
						Language: semantics.LanguageGo,
						Content:  source,
					})
				}(i)
			}
			wg.Wait()

			for i := 0; i < goroutines; i++ {
				Expect(errs[i]).NotTo(HaveOccurred())
				Expect(results[i].ParseStatus).To(Equal(semantics.ParseStatus("ok")))
			}
		})
	})
})

func expectImportsAndFindingsInDocumentOrder(analyzer *semantics.Analyzer) {
	source := []byte(`package main

import (
	"os"
	"fmt"
)

func NewZeta() *int {
	fmt.Println(os.Args)
	return nil
}

func NewAlpha() *int {
	return nil
}
`)

	result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
		Path:     "main.go",
		Language: semantics.LanguageGo,
		Content:  source,
	})
	Expect(err).NotTo(HaveOccurred())

	Expect(result.Imports).To(HaveLen(2))
	Expect(result.Imports[0].Location.StartByte).To(BeNumerically("<=", result.Imports[1].Location.StartByte))

	Expect(result.Findings).NotTo(BeEmpty())
	for i := 1; i < len(result.Findings); i++ {
		Expect(result.Findings[i].Location.StartByte).To(BeNumerically(">=", result.Findings[i-1].Location.StartByte))
	}
	names := map[string]bool{}
	for _, f := range result.Findings {
		names[f.Name] = true
	}
	Expect(names).To(HaveKey("NewZeta"))
	Expect(names).To(HaveKey("NewAlpha"))

	Expect(result.Findings[0].Name).To(Equal("NewZeta"))
}

var _ = Describe("consumer-facing lifecycle and safety", func() {
	It("exposes no Close method on Analyzer for callers to manage (AC-6.3)", func() {
		_, ok := reflect.TypeOf(&semantics.Analyzer{}).MethodByName("Close")
		Expect(ok).To(BeFalse())
	})
})
