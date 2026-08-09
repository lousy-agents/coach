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

func toctouGoFinding(name string, row uint) semantics.Finding {
	return semantics.Finding{
		Kind:     "toctou_check_then_act",
		Name:     name,
		Location: semantics.Location{StartRow: row, EndRow: row},
		Evidence: "if _, err := os.Stat(path); err == nil { os.Open(path) }",
	}
}

// resultForLanguage builds a clean *semantics.Result with an explicit
// Language, unlike the package-level cleanResult helper which always sets
// LanguageGo. Used where a spec must prove the rule's field shape holds for
// a Language other than Go.
func resultForLanguage(path string, lang semantics.Language, findings ...semantics.Finding) *semantics.Result {
	return &semantics.Result{Path: path, Language: lang, ParseStatus: "ok", Findings: findings}
}

var _ = Describe("security.toctou_check_then_act", func() {
	When("a TS/TSX finding reports an existsSync-then-act filesystem race", func() {
		It("emits exactly one signal with the locked field shape", func() {
			finding := toctouFinding("readConfig", 4)
			finding.Confidence = "high"
			finding.SuggestedSkill = "find-bugs"
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "src/a.ts", Status: "modified",
				Head: resultForLanguage("src/a.ts", semantics.LanguageTypeScript, finding),
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

	When("a toctou finding with the same identity is present in both base and head", func() {
		It("marks the resulting signal existing", func() {
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path:   "src/a.ts",
				Status: "modified",
				Base:   cleanResult("src/a.ts", toctouFinding("readConfig", 1)),
				Head:   cleanResult("src/a.ts", toctouFinding("readConfig", 9)),
			}}})

			Expect(report.Signals).To(HaveLen(1))
			signal := report.Signals[0]
			Expect(signal.RuleID).To(Equal("security.toctou_check_then_act"))
			Expect(signal.Lifecycle).To(Equal(codesignal.Lifecycle("existing")))
			Expect(signal.Fingerprint).NotTo(BeEmpty())
			Expect(signal.ID).NotTo(BeEmpty())
		})
	})

	When("a toctou finding is present only in head", func() {
		It("marks the resulting signal introduced", func() {
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path:   "src/a.ts",
				Status: "modified",
				Base:   cleanResult("src/a.ts"),
				Head:   cleanResult("src/a.ts", toctouFinding("readConfig", 4)),
			}}})

			Expect(report.Signals).To(HaveLen(1))
			signal := report.Signals[0]
			Expect(signal.RuleID).To(Equal("security.toctou_check_then_act"))
			Expect(signal.Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
		})
	})

	When("a toctou finding is present only in base and IncludeResolved is set", func() {
		It("marks the resulting signal resolved", func() {
			report := build(codesignal.Options{IncludeResolved: true}, codesignal.Input{Files: []codesignal.FileChange{{
				Path:   "src/a.ts",
				Status: "removed",
				Base:   cleanResult("src/a.ts", toctouFinding("readConfig", 4)),
			}}})

			Expect(report.Signals).To(HaveLen(1))
			signal := report.Signals[0]
			Expect(signal.RuleID).To(Equal("security.toctou_check_then_act"))
			Expect(signal.Lifecycle).To(Equal(codesignal.Lifecycle("resolved")))
		})
	})

	When("a Go finding reports an os.Stat-then-act filesystem race", func() {
		It("emits exactly one signal with the locked field shape, independent of language", func() {
			finding := toctouGoFinding("readFile", 4)
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "pkg/a.go", Status: "modified",
				Head: resultForLanguage("pkg/a.go", semantics.LanguageGo, finding),
			}}})

			Expect(report.Signals).To(HaveLen(1))
			signal := report.Signals[0]
			Expect(signal.RuleID).To(Equal("security.toctou_check_then_act"))
			Expect(signal.RuleVersion).To(Equal("1"))
			Expect(signal.Kind).To(Equal("toctou_check_then_act"))
			Expect(signal.Category).To(Equal(codesignal.Category("security")))
			Expect(signal.Severity).To(Equal(codesignal.Severity("medium")))
			Expect(signal.Confidence).To(Equal(codesignal.Confidence("medium")))
			Expect(signal.Path).To(Equal("pkg/a.go"))
			Expect(signal.Subject).To(Equal("readFile"))
			Expect(signal.Location).To(Equal(finding.Location))
			Expect(signal.Evidence).To(Equal(finding.Evidence))
			Expect(signal.Provenance).To(Equal(codesignal.Provenance{Producer: "semantics", FindingKind: "toctou_check_then_act"}))
		})
	})

	When("a toctou finding appears in a repository baseline report", func() {
		It("marks the signal baseline", func() {
			report := build(codesignal.Options{Baseline: true}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "src/a.ts", Status: "added",
				Head: cleanResult("src/a.ts", toctouFinding("readConfig", 4)),
			}}})

			Expect(report.Signals).To(HaveLen(1))
			signal := report.Signals[0]
			Expect(signal.RuleID).To(Equal("security.toctou_check_then_act"))
			Expect(signal.Lifecycle).To(Equal(codesignal.Lifecycle("baseline")))
			Expect(report.Summary.BaselineSignals).To(Equal(1))
		})
	})

	When("a toctou finding is present in head with no base result and Baseline is not set", func() {
		It("marks the resulting signal unknown", func() {
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "src/a.ts", Status: "modified",
				Head: cleanResult("src/a.ts", toctouFinding("readConfig", 4)),
			}}})

			Expect(report.Signals).To(HaveLen(1))
			signal := report.Signals[0]
			Expect(signal.RuleID).To(Equal("security.toctou_check_then_act"))
			Expect(signal.Lifecycle).To(Equal(codesignal.Lifecycle("unknown")))
		})
	})
})
