package codesignal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func toctouFinding(name string, row uint) semantics.Finding {
	return semantics.Finding{
		Kind:     "toctou_check_then_act",
		Name:     name,
		Location: semantics.Location{StartRow: row, EndRow: row},
		Evidence: "if (existsSync(path)) { readFileSync(path) }",
	}
}

var _ = Describe("security.toctou_check_then_act", func() {
	When("a TS/TSX finding reports an existsSync-then-act filesystem race", func() {
		It("emits exactly one signal with the locked field shape", func() {
			finding := toctouFinding("readConfig", 4)
			finding.Confidence = "high"
			finding.SuggestedSkill = "find-bugs"
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "src/a.ts", Status: "modified",
				Head: cleanResult("src/a.ts", finding),
			}}})

			Expect(report.Signals).To(HaveLen(1))
			signal := report.Signals[0]
			Expect(signal.RuleID).To(Equal("security.toctou_check_then_act"))
			Expect(signal.RuleVersion).To(Equal("1"))
			Expect(signal.Kind).To(Equal("toctou_check_then_act"))
			Expect(signal.Category).To(Equal(codesignal.Category("security")))
			Expect(signal.Severity).To(Equal(codesignal.Severity("medium")))
			Expect(signal.Confidence).To(Equal(codesignal.Confidence("high")))
			Expect(signal.Path).To(Equal("src/a.ts"))
			Expect(signal.Subject).To(Equal("readConfig"))
			Expect(signal.Location).To(Equal(finding.Location))
			Expect(signal.Evidence).To(Equal(finding.Evidence))
			Expect(signal.SuggestedSkill).To(Equal("find-bugs"))
			Expect(signal.WhyItMatters).To(ContainSubstring("CWE-367"))
			Expect(signal.Provenance).To(Equal(codesignal.Provenance{Producer: "semantics", FindingKind: "toctou_check_then_act"}))
		})
	})

	When("a toctou finding has no recommendation", func() {
		It("defaults the recommendation to the EAFP fix guidance", func() {
			finding := toctouFinding("readConfig", 4)
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "src/a.ts", Status: "modified",
				Head: cleanResult("src/a.ts", finding),
			}}})

			Expect(report.Signals).To(HaveLen(1))
			Expect(report.Signals[0].Recommendation).NotTo(BeEmpty())
		})
	})

	When("a toctou finding provides its own recommendation", func() {
		It("preserves it verbatim", func() {
			finding := toctouFinding("readConfig", 4)
			finding.Recommendation = "wrap the access in try/catch"
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "src/a.ts", Status: "modified",
				Head: cleanResult("src/a.ts", finding),
			}}})

			Expect(report.Signals).To(HaveLen(1))
			Expect(report.Signals[0].Recommendation).To(Equal("wrap the access in try/catch"))
		})
	})

	When("a file has no toctou_check_then_act findings", func() {
		It("emits no security.toctou_check_then_act signal", func() {
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "src/a.ts", Status: "modified",
				Head: cleanResult("src/a.ts", semantics.Finding{Kind: "not_a_real_kind", Name: "unrelated"}),
			}}})
			Expect(report.Signals).To(BeEmpty())
		})
	})
})
