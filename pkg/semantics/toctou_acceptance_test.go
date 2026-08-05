package semantics_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/semantics"
)

// analyzeTSToctou analyzes source as TypeScript and asserts it parsed
// cleanly, mirroring analyzeTSCC's shape for a different worked-example
// family (GitHub issue #177, Story 1).
func analyzeTSToctou(analyzer *semantics.Analyzer, source string) *semantics.Result {
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

func toctouFindings(findings []semantics.Finding) []semantics.Finding {
	var out []semantics.Finding
	for _, f := range findings {
		if f.Kind == "toctou_check_then_act" {
			out = append(out, f)
		}
	}
	return out
}

var _ = Describe("TOCTOU check-then-act detection (Story 1, CWE-367, GitHub issue #177)", func() {
	var analyzer *semantics.Analyzer

	BeforeEach(func() {
		analyzer = mustAnalyzer()
	})

	When("an if statement gates a matching fs act call behind a bare existsSync(path) check", func() {
		It("emits exactly one toctou_check_then_act Finding with the documented fields", func() {
			const source = `import { existsSync, readFileSync } from "fs";

function readIfPresent(p: string): string | undefined {
  if (existsSync(p)) {
    return readFileSync(p, "utf8");
  }
  return undefined;
}
`
			result := analyzeTSToctou(analyzer, source)
			findings := toctouFindings(result.Findings)
			Expect(findings).To(HaveLen(1), "expected exactly one toctou_check_then_act Finding")

			f := findings[0]
			Expect(f.Kind).To(Equal("toctou_check_then_act"))
			Expect(f.Confidence).To(Equal("medium"))
			Expect(f.SuggestedSkill).To(Equal("find-bugs"))
			Expect(f.Evidence).NotTo(BeEmpty())
			Expect(f.Recommendation).NotTo(BeEmpty())
			Expect(f.Recommendation).To(ContainSubstring("EAFP"))
			Expect(f.Recommendation).To(ContainSubstring("fs.promises.access"))
			Expect(f.Location.EndByte).To(BeNumerically(">", f.Location.StartByte))
		})
	})

	When("an if statement gates a matching fs act call behind a bare fs.existsSync(path) member-call check", func() {
		It("emits exactly one toctou_check_then_act Finding", func() {
			const source = `import * as fs from "fs";

function readIfPresent(p: string): string | undefined {
  if (fs.existsSync(p)) {
    return fs.readFileSync(p, "utf8");
  }
  return undefined;
}
`
			result := analyzeTSToctou(analyzer, source)
			Expect(toctouFindings(result.Findings)).To(HaveLen(1), "expected exactly one toctou_check_then_act Finding for the fs.existsSync member form")
		})
	})

	When("a while statement gates a matching fs act call behind a bare existsSync(path) check", func() {
		It("emits exactly one toctou_check_then_act Finding at the act call", func() {
			const source = `import { existsSync, unlinkSync } from "fs";

function removeWhenPresent(p: string): void {
  while (existsSync(p)) {
    unlinkSync(p);
  }
}
`
			result := analyzeTSToctou(analyzer, source)
			findings := toctouFindings(result.Findings)
			Expect(findings).To(HaveLen(1), "expected exactly one toctou_check_then_act Finding for the while form")

			f := findings[0]
			Expect(source[f.Location.StartByte:f.Location.EndByte]).To(ContainSubstring("unlinkSync(p)"))
		})
	})

	When("code uses EAFP style with no existsSync gate anywhere", func() {
		It("emits no toctou_check_then_act Finding", func() {
			const source = `import { readFileSync } from "fs";

function readOrDefault(p: string): string {
  try {
    return readFileSync(p, "utf8");
  } catch (e) {
    return "";
  }
}
`
			result := analyzeTSToctou(analyzer, source)
			Expect(toctouFindings(result.Findings)).To(BeEmpty(), "EAFP-style code with no existsSync gate must not be flagged")
		})
	})

	When("nested existsSync guards on the same path gate a single act call", func() {
		It("emits exactly one toctou_check_then_act Finding", func() {
			const source = `import { existsSync, readFileSync } from "fs";

function readIfPresent(p: string): string | undefined {
  if (existsSync(p)) {
    if (existsSync(p)) {
      return readFileSync(p, "utf8");
    }
  }
  return undefined;
}
`
			result := analyzeTSToctou(analyzer, source)
			Expect(toctouFindings(result.Findings)).To(HaveLen(1), "nested existsSync guards on the same path must dedupe to a single Finding on the act call")
		})
	})

	When("a TSX component gates a matching fs act call behind a bare existsSync(path) check", func() {
		It("emits exactly one toctou_check_then_act Finding", func() {
			const source = `import { existsSync, readFileSync } from "fs";

const Loader = (p: string) => {
  if (existsSync(p)) {
    const data = readFileSync(p, "utf8");
    return <div>{data}</div>;
  }
  return null;
};
`
			result := analyzeTSXToctou(analyzer, source)
			Expect(toctouFindings(result.Findings)).To(HaveLen(1), "expected exactly one toctou_check_then_act Finding for the TSX form")
		})
	})
})

var _ = Describe("Go TOCTOU check-then-act detection (Story 3, CWE-367, GitHub issue #179)", func() {
	var analyzer *semantics.Analyzer

	BeforeEach(func() {
		analyzer = mustAnalyzer()
	})

	When("an if statement's initializer Stat's a path and its err == nil consequence opens the same path", func() {
		It("emits exactly one toctou_check_then_act Finding located at the act call", func() {
			const source = `package main

import "os"

func readIfPresent(path string) ([]byte, error) {
	if _, err := os.Stat(path); err == nil {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return nil, err
	}
	return nil, nil
}
`
			result := analyzeGoToctou(analyzer, source)
			findings := toctouFindings(result.Findings)
			Expect(findings).To(HaveLen(1), "expected exactly one toctou_check_then_act Finding")

			f := findings[0]
			Expect(f.Kind).To(Equal("toctou_check_then_act"))
			Expect(f.Confidence).To(Equal("medium"))
			Expect(f.SuggestedSkill).To(Equal("find-bugs"))
			Expect(f.Evidence).NotTo(BeEmpty())
			Expect(f.Evidence).To(ContainSubstring("os.Stat(path)"))
			Expect(f.Evidence).To(ContainSubstring("os.Open(path)"))
			Expect(f.Recommendation).NotTo(BeEmpty())
			Expect(f.Recommendation).To(ContainSubstring("errors.Is(err, fs.ErrNotExist)"))
			Expect(f.Recommendation).To(ContainSubstring("os.Root"))
			Expect(f.Location.EndByte).To(BeNumerically(">", f.Location.StartByte))
			Expect(source[f.Location.StartByte:f.Location.EndByte]).To(Equal("os.Open(path)"))
		})
	})

	When("the if statement uses the inverse err != nil early-return gate", func() {
		It("emits no toctou_check_then_act Finding, even though the act call follows the if", func() {
			const source = `package main

import "os"

func readAfterCheck(path string) (*os.File, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	return os.Open(path)
}
`
			result := analyzeGoToctou(analyzer, source)
			Expect(toctouFindings(result.Findings)).To(BeEmpty(), "the err != nil inverse gate is out of scope for v1 and must not be flagged")
		})
	})

	When("the if statement's condition is a sentinel-error check instead of a direct nil comparison", func() {
		It("emits no toctou_check_then_act Finding", func() {
			const source = `package main

import (
	"errors"
	"io/fs"
	"os"
)

func openIfPresent(path string) (*os.File, error) {
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return os.Open(path)
	}
	return nil, nil
}
`
			result := analyzeGoToctou(analyzer, source)
			Expect(toctouFindings(result.Findings)).To(BeEmpty(), "a sentinel-error check like errors.Is is not the direct nil-comparison gate v1 requires")
		})
	})

	When("the code opens the file with ordinary error handling and no os.Stat/os.Lstat anywhere", func() {
		It("emits no toctou_check_then_act Finding", func() {
			const source = `package main

import "os"

func openPlain(path string) (*os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return f, nil
}
`
			result := analyzeGoToctou(analyzer, source)
			Expect(toctouFindings(result.Findings)).To(BeEmpty(), "EAFP-style code with no Stat/Lstat gate must not be flagged")
		})
	})

	When("nested Stat guards on the same path gate a single act call", func() {
		It("emits exactly one toctou_check_then_act Finding", func() {
			const source = `package main

import "os"

func f(path string) {
	if _, err := os.Stat(path); err == nil {
		if _, err := os.Stat(path); err == nil {
			os.Open(path)
		}
	}
}
`
			result := analyzeGoToctou(analyzer, source)
			Expect(toctouFindings(result.Findings)).To(HaveLen(1), "nested Stat guards on the same path must dedupe to a single Finding on the act call")
		})
	})
})
