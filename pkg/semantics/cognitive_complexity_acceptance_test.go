package semantics_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/semantics"
)

// goPackagePrefix wraps a Go function body fixture so AnalyzeBytes sees a
// valid compilation unit (package + imports used by the worked examples).
func goPackagePrefix(body string) []byte {
	return []byte("package main\n\nimport \"fmt\"\n\n" + body)
}

func analyzeGoCC(analyzer *semantics.Analyzer, body string) *semantics.Result {
	GinkgoHelper()
	result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
		Path:     "example.go",
		Language: semantics.LanguageGo,
		Content:  goPackagePrefix(body),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result).NotTo(BeNil())
	Expect(result.ParseStatus).To(Equal(semantics.ParseStatus("ok")))
	return result
}

func analyzeTSCC(analyzer *semantics.Analyzer, source string) *semantics.Result {
	GinkgoHelper()
	result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
		Path:     "example.ts",
		Language: semantics.LanguageTypeScript,
		Content:  []byte(source),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result).NotTo(BeNil())
	Expect(result.ParseStatus).To(Equal(semantics.ParseStatus("ok")))
	return result
}

func ccByName(records []semantics.FunctionCognitiveComplexity, name string) (semantics.FunctionCognitiveComplexity, bool) {
	for _, r := range records {
		if r.Name == name {
			return r, true
		}
	}
	return semantics.FunctionCognitiveComplexity{}, false
}

var _ = Describe("Cognitive Complexity scoring (Story 4 worked examples)", func() {
	var analyzer *semantics.Analyzer

	BeforeEach(func() {
		analyzer = mustAnalyzer()
	})

	Describe("Go worked examples", func() {
		When("AnalyzeBytes scores Example 1 (basic nesting + logical sequence)", func() {
			const source = `func processNumbers(numbers []int) {
	for _, num := range numbers {
		if num > 1 {
			if isOdd(num) && isValid(num) {
				fmt.Println(num)
			}
		}
	}
}
`

			It("shall attach processNumbers with score 7 and metrics max=7 sum=7", func() {
				result := analyzeGoCC(analyzer, source)

				rec, ok := ccByName(result.CognitiveComplexity, "processNumbers")
				Expect(ok).To(BeTrue(), "expected a cognitive_complexity record named processNumbers")
				Expect(rec.Score).To(Equal(7), "processNumbers Cognitive Complexity score")
				Expect(rec.Kind).To(Equal("function"))
				Expect(result.Metrics.MaxCognitiveComplexity).To(Equal(7))
				Expect(result.Metrics.SumCognitiveComplexity).To(Equal(7))
			})
		})

		When("AnalyzeBytes scores Example 2 (hybrid else if / else)", func() {
			const source = `func classify(n int) string {
	if n < 0 {
		return "negative"
	} else if n == 0 {
		return "zero"
	} else {
		if n%2 == 0 {
			return "even"
		}
		return "odd"
	}
}
`

			It("shall attach classify with score 5 and metrics max=5 sum=5", func() {
				result := analyzeGoCC(analyzer, source)

				rec, ok := ccByName(result.CognitiveComplexity, "classify")
				Expect(ok).To(BeTrue(), "expected a cognitive_complexity record named classify")
				Expect(rec.Score).To(Equal(5), "classify Cognitive Complexity score")
				Expect(rec.Kind).To(Equal("function"))
				Expect(result.Metrics.MaxCognitiveComplexity).To(Equal(5))
				Expect(result.Metrics.SumCognitiveComplexity).To(Equal(5))
			})
		})

		When("AnalyzeBytes scores Example 3 (switch single structural + nested if)", func() {
			const source = `func describe(code int) string {
	switch code {
	case 200:
		return "ok"
	case 404:
		return "not found"
	case 500:
		if isRetryable(code) {
			return "retry"
		}
		return "error"
	default:
		return "unknown"
	}
}
`

			It("shall attach describe with score 3 and metrics max=3 sum=3", func() {
				result := analyzeGoCC(analyzer, source)

				rec, ok := ccByName(result.CognitiveComplexity, "describe")
				Expect(ok).To(BeTrue(), "expected a cognitive_complexity record named describe")
				Expect(rec.Score).To(Equal(3), "describe Cognitive Complexity score")
				Expect(rec.Kind).To(Equal("function"))
				Expect(result.Metrics.MaxCognitiveComplexity).To(Equal(3))
				Expect(result.Metrics.SumCognitiveComplexity).To(Equal(3))
			})
		})

		When("AnalyzeBytes scores Example 4 (function literal raises nesting)", func() {
			const source = `func outer(items []string) {
	process := func(s string) {
		if len(s) > 0 {
			for _, c := range s {
				fmt.Print(c)
			}
		}
	}
	process("hello")
}
`

			It("shall attach outer=5 and nested process func_lit=3 with metrics max=5 sum=5 (sum top-level only)", func() {
				result := analyzeGoCC(analyzer, source)

				outer, ok := ccByName(result.CognitiveComplexity, "outer")
				Expect(ok).To(BeTrue(), "expected a cognitive_complexity record named outer")
				Expect(outer.Score).To(Equal(5), "outer Cognitive Complexity score")
				Expect(outer.Kind).To(Equal("function"))

				process, ok := ccByName(result.CognitiveComplexity, "process")
				Expect(ok).To(BeTrue(), "expected a cognitive_complexity record named process")
				Expect(process.Score).To(Equal(3), "process (func_lit) Cognitive Complexity score")
				Expect(process.Kind).To(Equal("func_lit"))

				Expect(result.Metrics.MaxCognitiveComplexity).To(Equal(5), "max over all records including nested")
				Expect(result.Metrics.SumCognitiveComplexity).To(Equal(5), "sum is top-level only; func_lit excluded")
			})
		})

		When("AnalyzeBytes scores Example 5 (logical sequences + labeled break)", func() {
			const source = `func search(matrix [][]int, target int) bool {
OUTER:
	for i, row := range matrix {
		for j, v := range row {
			if v == target && i > 0 || j > 0 {
				break OUTER
			}
		}
	}
	return false
}
`

			It("shall attach search with score 9 and metrics max=9 sum=9", func() {
				result := analyzeGoCC(analyzer, source)

				rec, ok := ccByName(result.CognitiveComplexity, "search")
				Expect(ok).To(BeTrue(), "expected a cognitive_complexity record named search")
				Expect(rec.Score).To(Equal(9), "search Cognitive Complexity score")
				Expect(rec.Kind).To(Equal("function"))
				Expect(result.Metrics.MaxCognitiveComplexity).To(Equal(9))
				Expect(result.Metrics.SumCognitiveComplexity).To(Equal(9))
			})
		})

		When("AnalyzeBytes scores Example 6 (direct recursion factorial)", func() {
			const source = `func factorial(n int) int {
	if n <= 1 {
		return 1
	}
	return n * factorial(n-1)
}
`

			It("shall attach factorial with score 2 and metrics max=2 sum=2", func() {
				result := analyzeGoCC(analyzer, source)

				rec, ok := ccByName(result.CognitiveComplexity, "factorial")
				Expect(ok).To(BeTrue(), "expected a cognitive_complexity record named factorial")
				Expect(rec.Score).To(Equal(2), "factorial Cognitive Complexity score")
				Expect(rec.Kind).To(Equal("function"))
				Expect(result.Metrics.MaxCognitiveComplexity).To(Equal(2))
				Expect(result.Metrics.SumCognitiveComplexity).To(Equal(2))
			})
		})
	})

	Describe("TS/TSX minimum analogs", func() {
		When("AnalyzeBytes scores TS-1 (nested if inside for with && sequence)", func() {
			const source = `function processNumbers(numbers: number[]) {
  for (const num of numbers) {
    if (num > 1) {
      if (isOdd(num) && isValid(num)) {
        console.log(num);
      }
    }
  }
}
`

			It("shall attach processNumbers with score 7 and metrics max=7 sum=7", func() {
				result := analyzeTSCC(analyzer, source)

				rec, ok := ccByName(result.CognitiveComplexity, "processNumbers")
				Expect(ok).To(BeTrue(), "expected a cognitive_complexity record named processNumbers")
				Expect(rec.Score).To(Equal(7), "TS processNumbers Cognitive Complexity score")
				Expect(rec.Kind).To(Equal("function"))
				Expect(result.Metrics.MaxCognitiveComplexity).To(Equal(7))
				Expect(result.Metrics.SumCognitiveComplexity).To(Equal(7))
			})
		})

		When("AnalyzeBytes scores TS-2 (if / else if / else with nested if)", func() {
			const source = `function classify(n: number): string {
  if (n < 0) {
    return "negative";
  } else if (n === 0) {
    return "zero";
  } else {
    if (n % 2 === 0) {
      return "even";
    }
    return "odd";
  }
}
`

			It("shall attach classify with score 5 and metrics max=5 sum=5", func() {
				result := analyzeTSCC(analyzer, source)

				rec, ok := ccByName(result.CognitiveComplexity, "classify")
				Expect(ok).To(BeTrue(), "expected a cognitive_complexity record named classify")
				Expect(rec.Score).To(Equal(5), "TS classify Cognitive Complexity score")
				Expect(rec.Kind).To(Equal("function"))
				Expect(result.Metrics.MaxCognitiveComplexity).To(Equal(5))
				Expect(result.Metrics.SumCognitiveComplexity).To(Equal(5))
			})
		})

		When("AnalyzeBytes scores TS-3 (arrow assigned to const raises nesting)", func() {
			const source = `function outer(items: string[]) {
  const process = (s: string) => {
    if (s.length > 0) {
      for (const c of s) {
        console.log(c);
      }
    }
  };
  process("hello");
}
`

			It("shall attach outer=5 and nested process arrow=3 with metrics max=5 sum=5", func() {
				result := analyzeTSCC(analyzer, source)

				outer, ok := ccByName(result.CognitiveComplexity, "outer")
				Expect(ok).To(BeTrue(), "expected a cognitive_complexity record named outer")
				Expect(outer.Score).To(Equal(5), "TS outer Cognitive Complexity score")
				Expect(outer.Kind).To(Equal("function"))

				process, ok := ccByName(result.CognitiveComplexity, "process")
				Expect(ok).To(BeTrue(), "expected a cognitive_complexity record named process")
				Expect(process.Score).To(Equal(3), "TS process (arrow) Cognitive Complexity score")
				Expect(process.Kind).To(Equal("arrow"))

				Expect(result.Metrics.MaxCognitiveComplexity).To(Equal(5), "max over all records including nested")
				Expect(result.Metrics.SumCognitiveComplexity).To(Equal(5), "sum is top-level only; arrow excluded")
			})
		})

		When("AnalyzeBytes scores TS mixed &&/|| boolean chain without Parent()-topmost over-count", func() {
			// a && b || c && d flattens to [&&, ||, &&] → 3 boolean runs (+3).
			// Plus the enclosing if (+1) → score 4. Must not double-charge a
			// right-hand && that gotreesitter parents under a phantom expression.
			const source = `function b() { if (a && b || c && d) {} }
`

			It("shall attach b with score 4 (if+1 + boolean runs+3)", func() {
				result := analyzeTSCC(analyzer, source)

				rec, ok := ccByName(result.CognitiveComplexity, "b")
				Expect(ok).To(BeTrue(), "expected a cognitive_complexity record named b")
				Expect(rec.Score).To(Equal(4), "TS mixed &&/|| chain Cognitive Complexity score")
				Expect(rec.Kind).To(Equal("function"))
				Expect(result.Metrics.MaxCognitiveComplexity).To(Equal(4))
				Expect(result.Metrics.SumCognitiveComplexity).To(Equal(4))
			})
		})
	})
})
