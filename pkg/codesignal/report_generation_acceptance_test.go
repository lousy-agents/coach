package codesignal_test

import (
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

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

	When("findings do not describe input mutation, gated kinds are below their density gate, and metrics are below their complexity thresholds", func() {
		It("does not raise any signal", func() {
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{Path: "state.go", Status: "modified", Head: &semantics.Result{
				Path: "state.go", Language: semantics.LanguageGo, ParseStatus: "ok",
				Findings: []semantics.Finding{
					{Kind: "constructor_func", Name: "NewState"},
					{Kind: "pointer_return", Name: "NewState"},
					{Kind: "unrelated", Name: "Elsewhere"},
				},
				Metrics: semantics.StructuralMetrics{MaxNestingDepth: 3, Ifs: 4, Fors: 3, ExprSwitches: 2, TypeSwitches: 1, Selects: 1},
			}}}})
			Expect(report.Signals).To(BeEmpty())
		})
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

func diagnostic(report *codesignal.Report, kind, path string) *codesignal.Diagnostic {
	for i := range report.Diagnostics {
		if report.Diagnostics[i].Kind == kind && report.Diagnostics[i].Path == path {
			return &report.Diagnostics[i]
		}
	}
	return nil
}
