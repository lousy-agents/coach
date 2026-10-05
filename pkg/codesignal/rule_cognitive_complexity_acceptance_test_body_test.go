package codesignal_test

import (
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func body_ruleCognitiveComplexityAcceptanceTest_shallEmitExactlyOneSignalPerSuchRecordWithTheLoc_46() {
	locA := semantics.Location{StartByte: 10, EndByte: 200, StartRow: 1, EndRow: 40}
	locB := semantics.Location{StartByte: 300, EndByte: 500, StartRow: 50, EndRow: 90}
	report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
		Path:   "complex.go",
		Status: "modified",
		Head: resultWithCognitiveComplexity("complex.go",
			ccRecord("simple", 14, semantics.Location{StartRow: 0, EndRow: 5}),
			ccRecord("hardOne", 15, locA),
			ccRecord("hardTwo", 22, locB),
		),
	}}})

	signals := signalsByRule(report, "complexity.cognitive_complexity")
	Expect(signals).To(HaveLen(2))

	bySubject := map[string]codesignal.Signal{}
	for _, s := range signals {
		bySubject[s.Subject] = s
	}

	one := bySubject["hardOne"]
	Expect(one.RuleID).To(Equal("complexity.cognitive_complexity"))
	Expect(one.RuleVersion).To(Equal("1"))
	Expect(one.Kind).To(Equal("cognitive_complexity"))
	Expect(one.Category).To(Equal(codesignal.Category("complexity")))
	Expect(one.Severity).To(Equal(codesignal.Severity("medium")))
	Expect(one.Confidence).To(Equal(codesignal.Confidence("high")))
	Expect(one.Path).To(Equal("complex.go"))
	Expect(one.Subject).To(Equal("hardOne"))
	Expect(one.Location).To(Equal(locA))
	Expect(one.Evidence).To(Equal("cognitive_complexity=15"))
	Expect(one.WhyItMatters).To(Equal(cognitiveComplexityWhyItMatters))
	Expect(one.Recommendation).To(Equal(cognitiveComplexityRecommendation))
	Expect(one.Provenance).To(Equal(codesignal.Provenance{Producer: "codesignal"}))

	two := bySubject["hardTwo"]
	Expect(two.Evidence).To(Equal("cognitive_complexity=22"))
	Expect(two.Location).To(Equal(locB))
	Expect(two.Path).To(Equal("complex.go"))
}

func body_ruleCognitiveComplexityAcceptanceTest_shallTreatScoreChurnWhileStill15AsResolvedPriorS_180() {
	base := resultWithCognitiveComplexity("complex.go",
		ccRecord("tangled", 16, semantics.Location{StartRow: 1, EndRow: 30}),
	)
	head := resultWithCognitiveComplexity("complex.go",
		ccRecord("tangled", 20, semantics.Location{StartRow: 1, EndRow: 35}),
	)

	defaultReport := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
		Path: "complex.go", Status: "modified", Base: base, Head: head,
	}}})
	active := signalsByRule(defaultReport, "complexity.cognitive_complexity")
	Expect(active).To(HaveLen(1))
	Expect(active[0].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
	Expect(active[0].Evidence).To(Equal("cognitive_complexity=20"))

	full := build(codesignal.Options{IncludeResolved: true}, codesignal.Input{Files: []codesignal.FileChange{{
		Path: "complex.go", Status: "modified", Base: base, Head: head,
	}}})
	all := signalsByRule(full, "complexity.cognitive_complexity")
	Expect(all).To(HaveLen(2))
	byLife := map[codesignal.Lifecycle]codesignal.Signal{}
	for _, s := range all {
		byLife[s.Lifecycle] = s
	}
	Expect(byLife[codesignal.Lifecycle("resolved")].Evidence).To(Equal("cognitive_complexity=16"))
	Expect(byLife[codesignal.Lifecycle("introduced")].Evidence).To(Equal("cognitive_complexity=20"))
}
