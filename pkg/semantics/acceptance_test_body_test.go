package semantics_test

import (
	"context"
	"errors"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func body_acceptanceTest_ordersImportsAndFindingsByDocumentPositionAC110_68(analyzer *semantics.Analyzer) {
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

func body_acceptanceTest_ordersSyntaxErrorsByDocumentPositionAC110_110(analyzer *semantics.Analyzer) {

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
}

func body_acceptanceTest_isSafeForConcurrentCallersAC19RunUnderGoTestRace_168(analyzer *semantics.Analyzer) {
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
}

func body_acceptanceTest_reportsALocationWhereStartByteEqualsEndByteWitho_240(analyzer *semantics.Analyzer) {

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
}

func body_acceptanceTest_whenSourceContainsEveryGoImportFormAC31AC32_283(analyze func(source string) *semantics.Result) {
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
}

func body_acceptanceTest_matchesTheDocumentedNewAZ09Pattern_391(name string, wantMatch bool, result *semantics.Result) {
	found := false
	for _, f := range result.Findings {
		if f.Kind == "constructor_func" && f.Name == name {
			found = true
		}
	}
	Expect(found).To(Equal(wantMatch))
}
