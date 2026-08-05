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
