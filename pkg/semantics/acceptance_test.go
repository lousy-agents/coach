package semantics_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/semantics"
)

// mustAnalyzer builds an Analyzer with default options, failing the spec
// immediately if construction fails (it never should for AnalyzerOptions{}).
func mustAnalyzer() *semantics.Analyzer {
	a, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
	Expect(err).NotTo(HaveOccurred())
	return a
}

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
			// MatchJSON only checks semantic equivalence (it would still pass
			// on differing key order or whitespace); AC-1.3 requires the
			// serialized bytes themselves to be identical.
			Expect(secondJSON).To(Equal(firstJSON))
		})

		It("orders imports and findings by document position (AC-1.10)", func() {
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
			// NewZeta is declared before NewAlpha in source, so its findings
			// must sort first regardless of how many findings each produces.
			Expect(result.Findings[0].Name).To(Equal("NewZeta"))
		})

		It("orders syntax errors by document position (AC-1.10)", func() {
			// Two separate unclosed-brace functions, each producing at least
			// one ERROR/MISSING node, so SyntaxErrors has more than one entry
			// to order.
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
			// Unterminated call argument list: Tree-sitter's Go grammar
			// recovers by inserting a zero-width MISSING ")" rather than an
			// ERROR node.
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

var _ = Describe("import, metric, and finding extraction", func() {
	var analyzer *semantics.Analyzer

	BeforeEach(func() {
		analyzer = mustAnalyzer()
	})

	analyze := func(source string) *semantics.Result {
		result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
			Path:     "main.go",
			Language: semantics.LanguageGo,
			Content:  []byte(source),
		})
		Expect(err).NotTo(HaveOccurred())
		return result
	}

	Context("when source contains every Go import form (AC-3.1, AC-3.2)", func() {
		var result *semantics.Result

		BeforeEach(func() {
			result = analyze(`package main

import (
	"fmt"
	o "os"
	. "strings"
	_ "unicode"
	` + "`unicode/utf8`" + `
)

func F() {}
`)
		})

		DescribeTable("extracts the import's path and alias",
			func(wantPath, wantAlias string) {
				var found *semantics.ImportFeature
				for i := range result.Imports {
					if result.Imports[i].Path == wantPath {
						found = &result.Imports[i]
						break
					}
				}
				Expect(found).NotTo(BeNil(), "expected an import with path %q, got %+v", wantPath, result.Imports)
				Expect(found.Alias).To(Equal(wantAlias))
			},
			Entry("plain single-quoted import", "fmt", ""),
			Entry("aliased import", "os", "o"),
			Entry("dot import", "strings", "."),
			Entry("blank import", "unicode", "_"),
			Entry("raw-string (backtick) import path", "unicode/utf8", ""),
		)
	})

	It("computes exact structural metric counts for every tracked branching construct (AC-3.3)", func() {
		result := analyze(`package main

func F(ch chan int) {
	if true {
	}
	if false {
	}
	for i := 0; i < 3; i++ {
	}
	switch 1 {
	case 1:
	}
	switch x := interface{}(1).(type) {
	case int:
		_ = x
	}
	select {
	case <-ch:
	}
}

type T struct{}

func (t T) M() {
}
`)

		Expect(result.Metrics).To(Equal(semantics.StructuralMetrics{
			Ifs: 2, Fors: 1, ExprSwitches: 1, TypeSwitches: 1, Selects: 1,
			Functions: 1, Methods: 1, MaxNestingDepth: result.Metrics.MaxNestingDepth,
		}))
	})

	Context("nesting depth (AC-3.4)", func() {
		It("counts the deepest nested block within a function body", func() {
			result := analyze(`package main

func F() {
	if true {
		if true {
		}
	}
}
`)
			Expect(result.Metrics.MaxNestingDepth).To(Equal(3))
		})

		It("reports 0 for a file with no functions", func() {
			result := analyze("package main\n")
			Expect(result.Metrics.MaxNestingDepth).To(Equal(0))
		})
	})

	Context("constructor-like function detection (AC-3.5)", func() {
		var result *semantics.Result

		BeforeEach(func() {
			result = analyze(`package main

func NewFoo() {}
func New() {}
func Newton() {}
`)
		})

		DescribeTable("matches the documented ^New([A-Z0-9_]|$) pattern",
			func(name string, wantMatch bool) {
				found := false
				for _, f := range result.Findings {
					if f.Kind == "constructor_func" && f.Name == name {
						found = true
					}
				}
				Expect(found).To(Equal(wantMatch))
			},
			Entry("NewFoo matches", "NewFoo", true),
			Entry("bare New matches", "New", true),
			Entry("Newton does not match", "Newton", false),
		)
	})

	It("detects pointer-returning functions and methods (AC-3.6)", func() {
		result := analyze(`package main

func NewThing() *int { return nil }

type T struct{}

func (t T) Get() *int { return nil }

func Value() int { return 0 }
`)

		names := map[string]bool{}
		for _, f := range result.Findings {
			if f.Kind == "pointer_return" {
				names[f.Name] = true
			}
		}
		Expect(names).To(HaveKey("NewThing"))
		Expect(names).To(HaveKey("Get"))
		Expect(names).NotTo(HaveKey("Value"))
	})
})

var _ = Describe("the frozen JSON contract", func() {
	It("serializes a result with stable snake_case field names a consumer can persist and later read back (AC-4.1, AC-4.3)", func() {
		analyzer := mustAnalyzer()
		result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
			Path:     "main.go",
			Language: semantics.LanguageGo,
			Content:  []byte("package main\n\nfunc NewFoo() *int {\n\tif true {\n\t}\n\treturn nil\n}\n"),
		})
		Expect(err).NotTo(HaveOccurred())

		persisted, err := json.Marshal(result)
		Expect(err).NotTo(HaveOccurred())

		var asMap map[string]json.RawMessage
		Expect(json.Unmarshal(persisted, &asMap)).To(Succeed())
		Expect(asMap).To(HaveKey("path"))
		Expect(asMap).To(HaveKey("language"))
		Expect(asMap).To(HaveKey("parse_status"))
		Expect(asMap).To(HaveKey("metrics"))

		// A future consumer reading a persisted result back must recover the
		// same facts -- this is the promise the frozen JSON shape exists to
		// keep, not merely that marshaling succeeds.
		var roundTripped semantics.Result
		Expect(json.Unmarshal(persisted, &roundTripped)).To(Succeed())
		Expect(roundTripped).To(Equal(*result))
	})
})

var _ = Describe("consumer-facing lifecycle and safety", func() {
	It("exposes no Close method on Analyzer for callers to manage (AC-6.3)", func() {
		_, ok := reflect.TypeOf(&semantics.Analyzer{}).MethodByName("Close")
		Expect(ok).To(BeFalse())
	})
})
