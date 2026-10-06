package codesignal_test

import (
	"encoding/json"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Deterministic report output", func() {
	It("serializes equivalent reordered input byte-identically", func() {
		first := codesignal.Input{Files: []codesignal.FileChange{{Path: "b.go", Status: "modified", Head: cleanResult("b.go", mutation("B", 2))}, {Path: "a.go", Status: "modified", Head: cleanResult("a.go", mutation("A", 1))}}}
		second := codesignal.Input{Files: []codesignal.FileChange{{Path: "a.go", Status: "modified", Head: cleanResult("a.go", mutation("A", 1))}, {Path: "b.go", Status: "modified", Head: cleanResult("b.go", mutation("B", 2))}}}
		left, err := json.Marshal(build(codesignal.Options{}, first))
		Expect(err).NotTo(HaveOccurred())
		right, err := json.Marshal(build(codesignal.Options{}, second))
		Expect(err).NotTo(HaveOccurred())
		Expect(left).To(Equal(right))
	})

	It("preserves a fingerprint while locations change and repeats IDs", func() {
		one := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{Path: "f.go", Status: "modified", Head: cleanResult("f.go", mutation("Update", 1))}}})
		two := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{Path: "f.go", Status: "modified", Head: cleanResult("f.go", mutation("Update", 99))}}})
		three := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{Path: "f.go", Status: "modified", Head: cleanResult("f.go", mutation("Update", 1))}}})
		Expect(two.Signals[0].Fingerprint).To(Equal(one.Signals[0].Fingerprint))
		Expect(three.Signals[0].ID).To(Equal(one.Signals[0].ID))
	})

	It("orders signals and diagnostics predictably", func() {
		base := cleanResult("f.go")
		head := cleanResult("f.go", mutation("Later", 8), mutation("Earlier", 2))
		report := build(codesignal.Options{}, codesignal.Input{
			Files:       []codesignal.FileChange{{Path: "f.go", Status: "modified", Base: base, Head: head, ChangedRanges: []codesignal.LineRange{{StartRow: 2, EndRow: 2}}}},
			Diagnostics: []codesignal.Diagnostic{{Path: "z.go", Kind: "custom", Message: "last"}, {Path: "a.go", Kind: "custom", Message: "first"}},
		})
		Expect(report.Signals).To(HaveLen(2))
		Expect(report.Signals[0].Subject).To(Equal("Earlier"))
		Expect(report.Signals[1].Subject).To(Equal("Later"))
		Expect(report.Diagnostics[0].Path).To(Equal("a.go"))
		Expect(report.Diagnostics[1].Path).To(Equal("z.go"))
	})

	It("orders every lifecycle and changed priority group deterministically", func() {
		introducedChanged := mutation("introduced changed", 1)
		introducedOutside := mutation("introduced outside", 2)
		existingChanged := mutation("existing changed", 3)
		existingOutside := mutation("existing outside", 4)
		resolved := mutation("resolved", 5)
		unknown := mutation("unknown", 6)
		report := build(codesignal.Options{IncludeResolved: true}, codesignal.Input{Files: []codesignal.FileChange{
			{Path: "introduced.go", Status: "modified", Base: cleanResult("introduced.go"), Head: cleanResult("introduced.go", introducedChanged, introducedOutside), ChangedRanges: []codesignal.LineRange{{StartRow: 1, EndRow: 1}}},
			{Path: "existing.go", Status: "modified", Base: cleanResult("existing.go", existingChanged, existingOutside), Head: cleanResult("existing.go", existingChanged, existingOutside), ChangedRanges: []codesignal.LineRange{{StartRow: 3, EndRow: 3}}},
			{Path: "resolved.go", Status: "removed", Base: cleanResult("resolved.go", resolved)},
			{Path: "unknown.go", Status: "modified", Head: cleanResult("unknown.go", unknown)},
		}})
		Expect(report.Signals).To(HaveLen(6))
		Expect([]string{
			report.Signals[0].Subject,
			report.Signals[1].Subject,
			report.Signals[2].Subject,
			report.Signals[3].Subject,
			report.Signals[4].Subject,
			report.Signals[5].Subject,
		}).To(Equal([]string{"introduced changed", "existing changed", "introduced outside", "existing outside", "resolved", "unknown"}))
	})

	It("orders equally prioritized signals by confidence, path, row, and column", func() {
		high := mutation("high confidence", 10)
		high.Confidence = "high"
		low := mutation("low confidence", 0)
		low.Confidence = "low"
		earlierRow := mutation("earlier row", 1)
		laterColumn := mutation("later column", 2)
		laterColumn.Location.StartCol = 4
		earlierColumn := mutation("earlier column", 2)
		earlierColumn.Location.StartCol = 1
		otherPath := mutation("other path", 0)
		report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{
			{Path: "z.go", Status: "modified", Base: cleanResult("z.go"), Head: cleanResult("z.go", high)},
			{Path: "a.go", Status: "modified", Base: cleanResult("a.go"), Head: cleanResult("a.go", laterColumn, earlierColumn, earlierRow)},
			{Path: "b.go", Status: "modified", Base: cleanResult("b.go"), Head: cleanResult("b.go", otherPath)},
			{Path: "low.go", Status: "modified", Base: cleanResult("low.go"), Head: cleanResult("low.go", low)},
		}})
		Expect([]string{
			report.Signals[0].Subject,
			report.Signals[1].Subject,
			report.Signals[2].Subject,
			report.Signals[3].Subject,
			report.Signals[4].Subject,
			report.Signals[5].Subject,
		}).To(Equal([]string{"high confidence", "earlier row", "earlier column", "later column", "other path", "low confidence"}))
	})

	It("sorts diagnostics by path, kind, location, and message", func() {
		atRowOne := semantics.Location{StartRow: 1}
		atRowTwo := semantics.Location{StartRow: 2}
		report := build(codesignal.Options{}, codesignal.Input{Diagnostics: []codesignal.Diagnostic{
			{Path: "b.go", Kind: "z", Message: "last"},
			{Path: "a.go", Kind: "z", Message: "later location", Location: &atRowTwo},
			{Path: "a.go", Kind: "z", Message: "earlier location", Location: &atRowOne},
			{Path: "a.go", Kind: "a", Message: "kind first"},
			{Path: "a.go", Kind: "z", Message: "no location first"},
		}})
		Expect(report.Diagnostics).To(HaveLen(5))
		Expect([]string{
			report.Diagnostics[0].Message,
			report.Diagnostics[1].Message,
			report.Diagnostics[2].Message,
			report.Diagnostics[3].Message,
			report.Diagnostics[4].Message,
		}).To(Equal([]string{"kind first", "no location first", "earlier location", "later location", "last"}))
	})
})
