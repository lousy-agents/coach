package codesignal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func ccFile(path string, score int) codesignal.FileChange {
	return codesignal.FileChange{
		Path:   path,
		Status: "modified",
		Head:   resultWithCognitiveComplexity(path, ccRecord("fn", score, semantics.Location{StartRow: 1, EndRow: 9})),
	}
}

func metricsFile(path string, metrics semantics.StructuralMetrics) codesignal.FileChange {
	return codesignal.FileChange{
		Path:   path,
		Status: "modified",
		Head:   &semantics.Result{Path: path, Language: semantics.LanguageGo, ParseStatus: "ok", Metrics: metrics},
	}
}

func evidenceAndPath(signals []codesignal.Signal) []string {
	out := make([]string, len(signals))
	for i, s := range signals {
		out[i] = s.Path + " " + s.Evidence
	}
	return out
}

var _ = Describe("Magnitude ranking of same-tier signals", func() {
	When("two cognitive_complexity signals share severity and confidence but path order is opposite to magnitude order", func() {
		It("orders the higher-score signal first", func() {
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{
				ccFile("a.go", 16),
				ccFile("z.go", 29),
			}})

			signals := signalsByRule(report, "complexity.cognitive_complexity")
			Expect(signals).To(HaveLen(2))
			Expect(signals[0].Severity).To(Equal(signals[1].Severity))
			Expect(signals[0].Confidence).To(Equal(signals[1].Confidence))
			Expect(evidenceAndPath(signals)).To(Equal([]string{
				"z.go cognitive_complexity=29",
				"a.go cognitive_complexity=16",
			}))
		})

		It("orders by magnitude when both findings stay in the same severity tier", func() {
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{
				ccFile("a.go", 16),
				ccFile("z.go", 25),
			}})

			signals := signalsByRule(report, "complexity.cognitive_complexity")
			Expect(signals).To(HaveLen(2))
			Expect(signals[0].Severity).To(Equal(signals[1].Severity))
			Expect(evidenceAndPath(signals)).To(Equal([]string{
				"z.go cognitive_complexity=25",
				"a.go cognitive_complexity=16",
			}))
		})

		It("orders by magnitude for signals classified against a base revision", func() {
			baseFile := func(path string) *semantics.Result { return resultWithCognitiveComplexity(path) }
			a, z := ccFile("a.go", 16), ccFile("z.go", 25)
			a.Base, z.Base = baseFile("a.go"), baseFile("z.go")

			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{a, z}})

			signals := signalsByRule(report, "complexity.cognitive_complexity")
			Expect(signals).To(HaveLen(2))
			Expect(signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
			Expect(signals[1].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
			Expect(evidenceAndPath(signals)).To(Equal([]string{
				"z.go cognitive_complexity=25",
				"a.go cognitive_complexity=16",
			}))
		})
	})

	When("a lower-magnitude signal is introduced and a higher-magnitude signal already existed", func() {
		It("ranks lifecycle group above magnitude", func() {
			// Path order and magnitude order both put a.go first, so only ranking
			// the lifecycle group above magnitude puts z.go first.
			a, z := ccFile("a.go", 120), ccFile("z.go", 16)
			a.Base = resultWithCognitiveComplexity("a.go", ccRecord("fn", 120, semantics.Location{StartRow: 1, EndRow: 9}))
			z.Base = resultWithCognitiveComplexity("z.go")

			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{a, z}})

			signals := signalsByRule(report, "complexity.cognitive_complexity")
			Expect(signals).To(HaveLen(2))
			Expect([]codesignal.Lifecycle{signals[0].Lifecycle, signals[1].Lifecycle}).To(
				ConsistOf(codesignal.Lifecycle("introduced"), codesignal.Lifecycle("existing")))
			Expect(signals[0].Changed).To(Equal(signals[1].Changed))
			Expect(evidenceAndPath(signals)).To(Equal([]string{
				"z.go cognitive_complexity=16",
				"a.go cognitive_complexity=120",
			}))
		})
	})

	When("branch_density and max_nesting_depth signals share severity and confidence", func() {
		It("never compares their magnitudes: each keeps its path position", func() {
			// branch_sum=14 is 14/12 of its threshold and max_nesting_depth=7 is
			// 7/4, but the two metrics are unlike and neither rule has a peer to
			// swap with, so path order decides.
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{
				metricsFile("a.go", semantics.StructuralMetrics{Ifs: 14}),
				metricsFile("z.go", semantics.StructuralMetrics{MaxNestingDepth: 7}),
			}})

			Expect(report.Signals).To(HaveLen(2))
			Expect(report.Signals[0].Severity).To(Equal(report.Signals[1].Severity))
			Expect(report.Signals[0].Confidence).To(Equal(report.Signals[1].Confidence))
			Expect(evidenceAndPath(report.Signals)).To(Equal([]string{
				"a.go branch_sum=14",
				"z.go max_nesting_depth=7",
			}))
		})
	})

	When("a tier mixes two peers of one metric rule with signals of other rules", func() {
		It("swaps only the peers by magnitude and leaves every other signal at its path position", func() {
			files := []codesignal.FileChange{
				metricsFile("a.go", semantics.StructuralMetrics{Ifs: 14}),
				{Path: "b.go", Status: "modified", Head: cleanResult("b.go", mutation("Update", 4))},
				metricsFile("c.go", semantics.StructuralMetrics{MaxNestingDepth: 5}),
				metricsFile("z.go", semantics.StructuralMetrics{Ifs: 36}),
			}

			report := build(codesignal.Options{}, codesignal.Input{Files: files})

			Expect(report.Signals).To(HaveLen(4))
			for _, signal := range report.Signals {
				Expect(signal.Severity).To(Equal(codesignal.Severity("medium")))
				Expect(signal.Confidence).To(Equal(codesignal.Confidence("medium")))
			}
			var got []string
			for _, signal := range report.Signals {
				got = append(got, signal.Path+" "+signal.RuleID)
			}
			Expect(got).To(Equal([]string{
				"z.go complexity.branch_density",
				"b.go state.hidden_input_mutation",
				"c.go complexity.max_nesting_depth",
				"a.go complexity.branch_density",
			}))
			Expect(report.Signals[0].Evidence).To(Equal("branch_sum=36"))
			Expect(report.Signals[3].Evidence).To(Equal("branch_sum=14"))
		})
	})

	When("a lower-ratio signal has higher confidence than a higher-ratio signal", func() {
		It("ranks confidence above magnitude", func() {
			// Path order (a.go first) and magnitude order (1.75 vs 1.07) both put
			// a.go first, so only ranking confidence above magnitude puts z.go first.
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{
				metricsFile("a.go", semantics.StructuralMetrics{MaxNestingDepth: 7}),
				ccFile("z.go", 16),
			}})

			Expect(report.Signals).To(HaveLen(2))
			Expect(report.Signals[0].Severity).To(Equal(report.Signals[1].Severity))
			Expect(report.Signals[0].Confidence).NotTo(Equal(report.Signals[1].Confidence))
			Expect(evidenceAndPath(report.Signals)).To(Equal([]string{
				"z.go cognitive_complexity=16",
				"a.go max_nesting_depth=7",
			}))
		})
	})

	When("signals have no numeric magnitude", func() {
		It("keeps path then location ordering for hidden_input_mutation signals", func() {
			zFile := codesignal.FileChange{Path: "z.go", Status: "modified", Head: cleanResult("z.go", mutation("Late", 3))}
			aLate := codesignal.FileChange{Path: "a.go", Status: "modified", Head: cleanResult("a.go", mutation("Late", 9), mutation("Early", 2))}

			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{zFile, aLate}})

			Expect(report.Signals).To(HaveLen(3))
			var got []string
			for _, s := range report.Signals {
				got = append(got, s.Path+" "+s.Subject)
			}
			Expect(got).To(Equal([]string{"a.go Early", "a.go Late", "z.go Late"}))
		})

		It("never moves a medium no-magnitude signal below a low-severity signal", func() {
			constructors := []semantics.Finding{
				{Kind: "constructor_func", Name: "NewA", Location: semantics.Location{StartRow: 1, EndRow: 1}},
				{Kind: "constructor_func", Name: "NewB", Location: semantics.Location{StartRow: 2, EndRow: 2}},
			}
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{
				{Path: "a.go", Status: "modified", Head: cleanResult("a.go", constructors...)},
				{Path: "z.go", Status: "modified", Head: cleanResult("z.go", mutation("Update", 4))},
			}})

			Expect(report.Signals).To(HaveLen(3))
			Expect(report.Signals[0].RuleID).To(Equal("state.hidden_input_mutation"))
			Expect(report.Signals[0].Path).To(Equal("z.go"))
			Expect(report.Signals[1].RuleID).To(Equal("structure.constructor_density"))
			Expect(report.Signals[2].RuleID).To(Equal("structure.constructor_density"))
		})

		It("keeps a no-magnitude signal at its path position beside a magnitude-bearing signal of another rule", func() {
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{
				{Path: "a.go", Status: "modified", Head: cleanResult("a.go", mutation("Update", 4))},
				metricsFile("z.go", semantics.StructuralMetrics{MaxNestingDepth: 5}),
			}})

			Expect(report.Signals).To(HaveLen(2))
			Expect(report.Signals[0].Severity).To(Equal(report.Signals[1].Severity))
			Expect(report.Signals[0].Confidence).To(Equal(report.Signals[1].Confidence))
			Expect(report.Signals[0].RuleID).To(Equal("state.hidden_input_mutation"))
			Expect(report.Signals[1].RuleID).To(Equal("complexity.max_nesting_depth"))
		})
	})
})
