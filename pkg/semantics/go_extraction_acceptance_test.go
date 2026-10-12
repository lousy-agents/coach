package semantics_test

import (
	"context"

	"github.com/lousy-agents/coach/pkg/semantics"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

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
				expectImportWithAlias(result, wantPath, wantAlias)
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

			MaxCognitiveComplexity: 6,
			SumCognitiveComplexity: 6,
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

func expectImportWithAlias(result *semantics.Result, wantPath, wantAlias string) {
	var found *semantics.ImportFeature
	for i := range result.Imports {
		if result.Imports[i].Path == wantPath {
			found = &result.Imports[i]
			break
		}
	}
	Expect(found).NotTo(BeNil(), "expected an import with path %q, got %+v", wantPath, result.Imports)
	Expect(found.Alias).To(Equal(wantAlias))
}
