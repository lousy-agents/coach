package codesignal_test

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func cleanResult(path string, findings ...semantics.Finding) *semantics.Result {
	return &semantics.Result{Path: path, Language: semantics.LanguageGo, ParseStatus: "ok", Findings: findings}
}

func mutation(name string, row uint) semantics.Finding {
	return semantics.Finding{Kind: "mutates_input", Name: name, Location: semantics.Location{StartRow: row, EndRow: row}, Evidence: "input.value = 1"}
}

func build(options codesignal.Options, input codesignal.Input) *codesignal.Report {
	builder, err := codesignal.New(options)
	Expect(err).NotTo(HaveOccurred())
	report, err := builder.Build(context.Background(), input)
	Expect(err).NotTo(HaveOccurred())
	return report
}

func diagnostic(report *codesignal.Report, kind, path string) *codesignal.Diagnostic {
	for i := range report.Diagnostics {
		if report.Diagnostics[i].Kind == kind && report.Diagnostics[i].Path == path {
			return &report.Diagnostics[i]
		}
	}
	return nil
}

var _ = Describe("CodeSignal report generation", func() {
	When("caller-owned input is mutated", func() {
		It("emits actionable hidden-input-mutation feedback", func() {
			finding := mutation("Update", 4)
			finding.Confidence = "high"
			finding.SuggestedSkill = "go-testable-design"
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "state.go", Status: "modified", Head: cleanResult("state.go", finding),
			}}})

			Expect(report.Signals).To(HaveLen(1))
			signal := report.Signals[0]
			Expect(signal.RuleID).To(Equal("state.hidden_input_mutation"))
			Expect(signal.RuleVersion).To(Equal("1"))
			Expect(signal.Kind).To(Equal("hidden_input_mutation"))
			Expect(signal.Category).To(Equal(codesignal.Category("state_management")))
			Expect(signal.Severity).To(Equal(codesignal.Severity("medium")))
			Expect(signal.Confidence).To(Equal(codesignal.Confidence("high")))
			Expect(signal.Lifecycle).To(Equal(codesignal.Lifecycle("unknown")))
			Expect(signal.Path).To(Equal("state.go"))
			Expect(signal.Subject).To(Equal("Update"))
			Expect(signal.Evidence).To(Equal("input.value = 1"))
			Expect(signal.Recommendation).To(Equal("Return the updated value instead of mutating caller-owned state, or make the in-place behavior explicit through the API name and documentation."))
			Expect(signal.SuggestedSkill).To(Equal("go-testable-design"))
			Expect(signal.ID).NotTo(BeEmpty())
			Expect(signal.Fingerprint).NotTo(BeEmpty())
			Expect(signal.WhyItMatters).To(Equal("Mutating a caller-owned input can create behavior that is not visible from the function signature, make outcomes dependent on call ordering, introduce temporal coupling, make tests and local reasoning more difficult, and surprise callers that expect an input to remain unchanged."))
			Expect(signal.Location).To(Equal(finding.Location))
			Expect(signal.Provenance).To(Equal(codesignal.Provenance{Producer: "semantics", FindingKind: "mutates_input"}))
		})
	})

	When("a mutation finding provides optional coaching metadata", func() {
		It("preserves its recommendation and defaults an absent confidence to medium", func() {
			finding := mutation("Update", 4)
			finding.Recommendation = "copy the input before updating it"
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "state.go", Status: "modified", Head: cleanResult("state.go", finding),
			}}})

			Expect(report.Signals).To(HaveLen(1))
			Expect(report.Signals[0].Confidence).To(Equal(codesignal.Confidence("medium")))
			Expect(report.Signals[0].Recommendation).To(Equal("copy the input before updating it"))
		})
	})

	When("findings do not describe input mutation", func() {
		It("does not raise a hidden-input-mutation signal", func() {
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{Path: "state.go", Status: "modified", Head: cleanResult("state.go",
				semantics.Finding{Kind: "constructor_func", Name: "NewState"},
				semantics.Finding{Kind: "pointer_return", Name: "NewState"},
				semantics.Finding{Kind: "unrelated", Name: "Elsewhere"},
			)}}})
			Expect(report.Signals).To(BeEmpty())
		})
	})
})

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

var _ = Describe("Report diagnostics", func() {
	It("reports syntax locations while preserving other files' signals", func() {
		broken := &semantics.Result{Path: "broken.go", ParseStatus: "syntax_errors", SyntaxErrors: []semantics.SyntaxIssue{{Kind: "error", Location: semantics.Location{StartRow: 2, StartCol: 1, EndRow: 2, EndCol: 4}}}}
		report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{Path: "broken.go", Status: "modified", Head: broken}, {Path: "good.go", Status: "modified", Head: cleanResult("good.go", mutation("Update", 1))}}})
		diag := diagnostic(report, "syntax_errors", "broken.go")
		Expect(diag).NotTo(BeNil())
		Expect(diag.Location).NotTo(BeNil())
		Expect(*diag.Location).To(Equal(broken.SyntaxErrors[0].Location))
		Expect(report.Signals).To(HaveLen(1))
		Expect(report.Signals[0].Path).To(Equal("good.go"))
	})

	It("reports missing head results for added and modified files", func() {
		for _, status := range []codesignal.ChangeStatus{"added", "modified"} {
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{Path: string(status) + ".go", Status: status}}})
			Expect(diagnostic(report, "missing_head_result", string(status)+".go")).NotTo(BeNil())
		}
	})

	It("reports unsupported parse status and continues", func() {
		report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{Path: "odd.go", Status: "modified", Head: &semantics.Result{Path: "odd.go", ParseStatus: "other"}}, {Path: "good.go", Status: "modified", Head: cleanResult("good.go", mutation("Update", 1))}}})
		Expect(diagnostic(report, "unsupported_parse_status", "odd.go")).NotTo(BeNil())
		Expect(report.Signals).To(HaveLen(1))
	})

	It("processes clean analysis without a syntax diagnostic", func() {
		report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{Path: "good.go", Status: "modified", Head: cleanResult("good.go", mutation("Update", 1))}}})
		Expect(diagnostic(report, "syntax_errors", "good.go")).To(BeNil())
		Expect(report.Signals).To(HaveLen(1))
	})
})

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
