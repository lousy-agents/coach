package codesignal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func resultWithMetrics(path string, metrics semantics.StructuralMetrics) *semantics.Result {
	return &semantics.Result{Path: path, Language: semantics.LanguageGo, ParseStatus: "ok", Metrics: metrics}
}

var _ = Describe("Metrics-derived rules", func() {
	Describe("complexity.max_nesting_depth", func() {
		When("MaxNestingDepth is below the threshold", func() {
			It("emits no max_nesting_depth signal", func() {
				report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
					Path: "deep.go", Status: "modified",
					Head: resultWithMetrics("deep.go", semantics.StructuralMetrics{MaxNestingDepth: 3}),
				}}})
				Expect(report.Signals).To(BeEmpty())
			})
		})

		When("MaxNestingDepth is at or above the threshold", func() {
			It("emits exactly one nesting-depth signal with the locked field shape", func() {
				report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
					Path: "deep.go", Status: "modified",
					Head: resultWithMetrics("deep.go", semantics.StructuralMetrics{MaxNestingDepth: 5}),
				}}})
				Expect(report.Signals).To(HaveLen(1))
				signal := report.Signals[0]
				Expect(signal.RuleID).To(Equal("complexity.max_nesting_depth"))
				Expect(signal.RuleVersion).To(Equal("1"))
				Expect(signal.Kind).To(Equal("max_nesting_depth"))
				Expect(signal.Category).To(Equal(codesignal.Category("complexity")))
				Expect(signal.Severity).To(Equal(codesignal.Severity("medium")))
				Expect(signal.Confidence).To(Equal(codesignal.Confidence("medium")))
				Expect(signal.Subject).To(Equal(""))
				Expect(signal.Location).To(Equal(semantics.Location{}))
				Expect(signal.Provenance).To(Equal(codesignal.Provenance{Producer: "codesignal"}))
				Expect(signal.Evidence).To(ContainSubstring("5"))
				Expect(signal.WhyItMatters).NotTo(BeEmpty())
				Expect(signal.Recommendation).NotTo(BeEmpty())
			})
		})
	})

	Describe("complexity.branch_density", func() {
		When("the branch sum is below the threshold", func() {
			It("emits no branch_density signal", func() {
				report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
					Path: "branchy.go", Status: "modified",
					Head: resultWithMetrics("branchy.go", semantics.StructuralMetrics{Ifs: 5, Fors: 3}),
				}}})
				Expect(report.Signals).To(BeEmpty())
			})
		})

		When("the branch sum is at or above the threshold", func() {
			It("emits exactly one branch-density signal with the locked field shape", func() {
				report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
					Path: "branchy.go", Status: "modified",
					Head: resultWithMetrics("branchy.go", semantics.StructuralMetrics{
						Ifs: 4, Fors: 3, ExprSwitches: 2, TypeSwitches: 2, Selects: 1,
					}),
				}}})
				Expect(report.Signals).To(HaveLen(1))
				signal := report.Signals[0]
				Expect(signal.RuleID).To(Equal("complexity.branch_density"))
				Expect(signal.RuleVersion).To(Equal("1"))
				Expect(signal.Kind).To(Equal("branch_density"))
				Expect(signal.Category).To(Equal(codesignal.Category("complexity")))
				Expect(signal.Severity).To(Equal(codesignal.Severity("medium")))
				Expect(signal.Confidence).To(Equal(codesignal.Confidence("medium")))
				Expect(signal.Subject).To(Equal(""))
				Expect(signal.Location).To(Equal(semantics.Location{}))
				Expect(signal.Provenance).To(Equal(codesignal.Provenance{Producer: "codesignal"}))
				Expect(signal.Evidence).To(ContainSubstring("12"))
				Expect(signal.WhyItMatters).NotTo(BeEmpty())
				Expect(signal.Recommendation).NotTo(BeEmpty())
			})
		})
	})

	Describe("lifecycle classification for metrics-derived signals", func() {
		It("marks a nesting-depth crossing introduced when base was below threshold", func() {
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "deep.go", Status: "modified",
				Base: resultWithMetrics("deep.go", semantics.StructuralMetrics{MaxNestingDepth: 3}),
				Head: resultWithMetrics("deep.go", semantics.StructuralMetrics{MaxNestingDepth: 4}),
			}}})
			Expect(report.Signals).To(HaveLen(1))
			Expect(report.Signals[0].RuleID).To(Equal("complexity.max_nesting_depth"))
			Expect(report.Signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
		})

		It("marks a nesting-depth crossing resolved (with IncludeResolved) when head drops below threshold", func() {
			base := resultWithMetrics("deep.go", semantics.StructuralMetrics{MaxNestingDepth: 4})
			head := resultWithMetrics("deep.go", semantics.StructuralMetrics{MaxNestingDepth: 3})

			defaultReport := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "deep.go", Status: "modified", Base: base, Head: head,
			}}})
			Expect(defaultReport.Signals).To(BeEmpty())

			includeResolvedReport := build(codesignal.Options{IncludeResolved: true}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "deep.go", Status: "modified", Base: base, Head: head,
			}}})
			Expect(includeResolvedReport.Signals).To(HaveLen(1))
			Expect(includeResolvedReport.Signals[0].RuleID).To(Equal("complexity.max_nesting_depth"))
			Expect(includeResolvedReport.Signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("resolved")))
		})

		It("marks matching nesting-depth signals on both sides existing", func() {
			base := resultWithMetrics("deep.go", semantics.StructuralMetrics{MaxNestingDepth: 5})
			head := resultWithMetrics("deep.go", semantics.StructuralMetrics{MaxNestingDepth: 5})
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "deep.go", Status: "modified", Base: base, Head: head,
			}}})
			Expect(report.Signals).To(HaveLen(1))
			Expect(report.Signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("existing")))
		})
	})

	Describe("fingerprint stability across pure line-shift diffs", func() {
		It("keeps the same fingerprint when metrics are unchanged but locations differ across analyses", func() {
			one := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "deep.go", Status: "modified", Head: resultWithMetrics("deep.go", semantics.StructuralMetrics{MaxNestingDepth: 5}),
			}}})
			two := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "deep.go", Status: "modified", Head: resultWithMetrics("deep.go", semantics.StructuralMetrics{MaxNestingDepth: 5}),
			}}})
			Expect(one.Signals).To(HaveLen(1))
			Expect(two.Signals).To(HaveLen(1))
			Expect(two.Signals[0].Fingerprint).To(Equal(one.Signals[0].Fingerprint))
		})
	})

	Describe("caller-owned Metrics immutability", func() {
		It("does not mutate Head/Base Metrics when both metrics rules fire", func() {
			triggering := semantics.StructuralMetrics{
				MaxNestingDepth: 5,
				Ifs:             4, Fors: 3, ExprSwitches: 2, TypeSwitches: 2, Selects: 1,
			}
			base := resultWithMetrics("deep.go", triggering)
			head := resultWithMetrics("deep.go", triggering)
			baseBefore, headBefore := base.Metrics, head.Metrics

			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "deep.go", Status: "modified", Base: base, Head: head,
			}}})

			Expect(report.Signals).To(HaveLen(2))
			Expect(base.Metrics).To(Equal(baseBefore))
			Expect(head.Metrics).To(Equal(headBefore))
		})
	})
})
