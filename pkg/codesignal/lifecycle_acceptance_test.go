package codesignal_test

import (
	"github.com/lousy-agents/coach/pkg/codesignal"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Lifecycle classification", func() {
	It("marks the same signal in base and head existing", func() {
		report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{Path: "f.go", Status: "modified", Base: cleanResult("f.go", mutation("Update", 1)), Head: cleanResult("f.go", mutation("Update", 9))}}})
		Expect(report.Signals).To(HaveLen(1))
		Expect(report.Signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("existing")))
	})

	It("marks a signal present only in head introduced", func() {
		report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{Path: "f.go", Status: "modified", Base: cleanResult("f.go"), Head: cleanResult("f.go", mutation("Update", 1))}}})
		Expect(report.Signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
	})

	It("marks a signal removed from a deleted file resolved", func() {
		report := build(codesignal.Options{IncludeResolved: true}, codesignal.Input{Files: []codesignal.FileChange{{Path: "gone.go", Status: "removed", Base: cleanResult("gone.go", mutation("Update", 1))}}})
		Expect(report.Signals).To(HaveLen(1))
		Expect(report.Signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("resolved")))
	})

	It("marks a base-only signal resolved when the file remains present", func() {
		report := build(codesignal.Options{IncludeResolved: true}, codesignal.Input{Files: []codesignal.FileChange{{
			Path: "f.go", Status: "modified", Base: cleanResult("f.go", mutation("Update", 1)), Head: cleanResult("f.go"),
		}}})
		Expect(report.Signals).To(HaveLen(1))
		Expect(report.Signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("resolved")))
	})

	It("marks head signals unknown when no base result is available", func() {
		report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{Path: "f.go", Status: "modified", Head: cleanResult("f.go", mutation("Update", 1))}}})
		Expect(report.Signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("unknown")))
	})

	It("marks head signals introduced when the file is added", func() {
		report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{Path: "f.go", Status: "added", Head: cleanResult("f.go", mutation("Update", 1))}}})
		Expect(report.Signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
		Expect(report.Summary.IntroducedSignals).To(Equal(1))
		Expect(report.Summary.UnknownSignals).To(Equal(0))
	})

	It("keeps added-file signals as baseline in a repository baseline run", func() {
		report := build(codesignal.Options{Baseline: true}, codesignal.Input{Files: []codesignal.FileChange{{Path: "f.go", Status: "added", Head: cleanResult("f.go", mutation("Update", 1))}}})
		Expect(report.Signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("baseline")))
		Expect(report.Summary.BaselineSignals).To(Equal(1))
		Expect(report.Summary.IntroducedSignals).To(Equal(0))
	})

	It("leaves duplicate head occurrences beyond the base count unknown", func() {
		base := mutation("Update", 1)
		head := mutation("Update", 9)
		report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{Path: "f.go", Status: "modified", Base: cleanResult("f.go", base), Head: cleanResult("f.go", head, head)}}})
		Expect(report.Signals).To(HaveLen(2))
		Expect([]codesignal.Lifecycle{report.Signals[0].Lifecycle, report.Signals[1].Lifecycle}).To(ConsistOf(codesignal.Lifecycle("existing"), codesignal.Lifecycle("unknown")))
	})

	It("retains resolved lifecycle accounting when resolved signals are hidden", func() {
		report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{Path: "gone.go", Status: "removed", Base: cleanResult("gone.go", mutation("Update", 1))}}})
		Expect(report.Signals).To(BeEmpty())
		Expect(report.Summary.ResolvedSignals).To(Equal(1))
		Expect(report.Summary.ActiveSignals).To(Equal(0))
	})
})

var _ = Describe("Changed-line relevance", func() {
	It("marks inclusive zero-based overlap as changed", func() {
		report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{Path: "f.go", Status: "modified", Head: cleanResult("f.go", mutation("Update", 10)), ChangedRanges: []codesignal.LineRange{{StartRow: 10, EndRow: 12}}}}})
		Expect(report.Signals[0].Changed).To(BeTrue())
	})

	It("leaves a non-overlapping signal unchanged", func() {
		report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{Path: "f.go", Status: "modified", Head: cleanResult("f.go", mutation("Update", 9)), ChangedRanges: []codesignal.LineRange{{StartRow: 10, EndRow: 12}}}}})
		Expect(report.Signals[0].Changed).To(BeFalse())
	})

	It("diagnoses invalid ranges without changing lifecycle classification", func() {
		report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{Path: "f.go", Status: "modified", Head: cleanResult("f.go", mutation("Update", 1)), ChangedRanges: []codesignal.LineRange{{StartRow: 3, EndRow: 2}}}}})
		Expect(diagnostic(report, "invalid_changed_range", "f.go")).NotTo(BeNil())
		Expect(report.Signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("unknown")))
		Expect(report.Signals[0].Changed).To(BeFalse())
	})

	It("keeps changed-line relevance independent from an existing lifecycle", func() {
		report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
			Path:          "f.go",
			Status:        "modified",
			Base:          cleanResult("f.go", mutation("Update", 1)),
			Head:          cleanResult("f.go", mutation("Update", 10)),
			ChangedRanges: []codesignal.LineRange{{StartRow: 10, EndRow: 10}},
		}}})
		Expect(report.Signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("existing")))
		Expect(report.Signals[0].Changed).To(BeTrue())
	})
})
