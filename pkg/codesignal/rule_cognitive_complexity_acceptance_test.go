package codesignal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

const (
	cognitiveComplexityWhyItMatters   = "High cognitive complexity means the control flow takes more mental effort to follow: nested branches, mixed logical sequences, and jumps compound so reviewers and authors miss paths."
	cognitiveComplexityRecommendation = "Extract nested branches into named helpers, replace nested conditionals with early returns or lookup tables, and simplify boolean expressions so each function stays linearly readable."
)

func resultWithCognitiveComplexity(path string, records ...semantics.FunctionCognitiveComplexity) *semantics.Result {
	return &semantics.Result{
		Path:                path,
		Language:            semantics.LanguageGo,
		ParseStatus:         "ok",
		CognitiveComplexity: records,
	}
}

func ccRecord(name string, score int, loc semantics.Location) semantics.FunctionCognitiveComplexity {
	return semantics.FunctionCognitiveComplexity{
		Name:     name,
		Kind:     "function",
		Location: loc,
		Score:    score,
	}
}

func signalsByRule(report *codesignal.Report, ruleID string) []codesignal.Signal {
	var out []codesignal.Signal
	for _, s := range report.Signals {
		if s.RuleID == ruleID {
			out = append(out, s)
		}
	}
	return out
}

var _ = Describe("Story 2: complexity.cognitive_complexity codesignal rule", func() {
	When("a head result has one or more cognitive_complexity records with score >= 15", func() {
		It("shall emit exactly one signal per such record with the locked field shape", func() {
			body_ruleCognitiveComplexityAcceptanceTest_shallEmitExactlyOneSignalPerSuchRecordWithTheLoc_46()
		})
	})

	When("no record meets the threshold", func() {
		It("shall emit no complexity.cognitive_complexity signal", func() {
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path:   "simple.go",
				Status: "modified",
				Head: resultWithCognitiveComplexity("simple.go",
					ccRecord("a", 0, semantics.Location{}),
					ccRecord("b", 14, semantics.Location{StartRow: 1}),
				),
			}}})
			Expect(signalsByRule(report, "complexity.cognitive_complexity")).To(BeEmpty())
		})
	})

	When("cognitive_complexity is absent or empty on a Result", func() {
		It("shall emit no cognitive-complexity signals and shall not fail the build", func() {
			absent := &semantics.Result{
				Path:        "legacy.go",
				Language:    semantics.LanguageGo,
				ParseStatus: "ok",
			}
			empty := resultWithCognitiveComplexity("empty.go")

			absentReport := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "legacy.go", Status: "modified", Head: absent,
			}}})
			Expect(absentReport).NotTo(BeNil())
			Expect(signalsByRule(absentReport, "complexity.cognitive_complexity")).To(BeEmpty())

			emptyReport := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "empty.go", Status: "modified", Head: empty,
			}}})
			Expect(emptyReport).NotTo(BeNil())
			Expect(signalsByRule(emptyReport, "complexity.cognitive_complexity")).To(BeEmpty())
		})
	})

	Describe("lifecycle classification", func() {
		It("shall mark a function introduced when head crosses the threshold and base did not", func() {
			base := resultWithCognitiveComplexity("complex.go",
				ccRecord("tangled", 14, semantics.Location{StartRow: 1, EndRow: 20}),
			)
			head := resultWithCognitiveComplexity("complex.go",
				ccRecord("tangled", 15, semantics.Location{StartRow: 1, EndRow: 22}),
			)
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "complex.go", Status: "modified", Base: base, Head: head,
			}}})
			signals := signalsByRule(report, "complexity.cognitive_complexity")
			Expect(signals).To(HaveLen(1))
			Expect(signals[0].Subject).To(Equal("tangled"))
			Expect(signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
			Expect(signals[0].Evidence).To(Equal("cognitive_complexity=15"))
		})

		It("shall mark matching over-threshold records on both sides existing", func() {
			rec := ccRecord("tangled", 18, semantics.Location{StartRow: 1, EndRow: 30})
			base := resultWithCognitiveComplexity("complex.go", rec)
			head := resultWithCognitiveComplexity("complex.go", rec)
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "complex.go", Status: "modified", Base: base, Head: head,
			}}})
			signals := signalsByRule(report, "complexity.cognitive_complexity")
			Expect(signals).To(HaveLen(1))
			Expect(signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("existing")))
		})

		It("shall mark a function resolved (with IncludeResolved) when head drops below threshold", func() {
			base := resultWithCognitiveComplexity("complex.go",
				ccRecord("tangled", 20, semantics.Location{StartRow: 1, EndRow: 30}),
			)
			head := resultWithCognitiveComplexity("complex.go",
				ccRecord("tangled", 10, semantics.Location{StartRow: 1, EndRow: 15}),
			)

			defaultReport := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "complex.go", Status: "modified", Base: base, Head: head,
			}}})
			Expect(signalsByRule(defaultReport, "complexity.cognitive_complexity")).To(BeEmpty())

			includeResolvedReport := build(codesignal.Options{IncludeResolved: true}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "complex.go", Status: "modified", Base: base, Head: head,
			}}})
			signals := signalsByRule(includeResolvedReport, "complexity.cognitive_complexity")
			Expect(signals).To(HaveLen(1))
			Expect(signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("resolved")))
			Expect(signals[0].Evidence).To(Equal("cognitive_complexity=20"))
		})

		// Spec Story 2: evidence includes the numeric score, so score churn
		// while still over threshold yields distinct lifecycle keys.
		It("shall treat score churn while still >= 15 as resolved prior score plus introduced new score", func() {
			body_ruleCognitiveComplexityAcceptanceTest_shallTreatScoreChurnWhileStill15AsResolvedPriorS_180()
		})
	})
})
