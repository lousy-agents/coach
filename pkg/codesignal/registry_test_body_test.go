package codesignal_test

import (
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func body_registryTest_emitsOneStructureConstructorDensitySignalPerFind_48() {
	findings := []semantics.Finding{constructorFunc("NewA", 1), constructorFunc("NewB", 2), constructorFunc("NewC", 3)}
	report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
		Path: "ctor.go", Status: "modified", Head: cleanResult("ctor.go", findings...),
	}}})

	Expect(report.Signals).To(HaveLen(3))
	for _, signal := range report.Signals {
		Expect(signal.RuleID).To(Equal("structure.constructor_density"))
		Expect(signal.Kind).To(Equal("constructor_density"))
		Expect(signal.Category).To(Equal(codesignal.Category("structure")))
		Expect(signal.Severity).To(Equal(codesignal.Severity("low")))
		Expect(signal.Confidence).To(Equal(codesignal.Confidence("medium")))
		Expect(signal.RuleVersion).To(Equal("1"))
		Expect(signal.WhyItMatters).NotTo(BeEmpty())
		Expect(signal.Recommendation).NotTo(BeEmpty())
		Expect(signal.Provenance.FindingKind).To(Equal("constructor_func"))
	}
}

func body_registryTest_emitsOneStructurePointerReturnDensitySignalPerFi_79() {
	findings := []semantics.Finding{pointerReturn("NewA", 1), pointerReturn("NewB", 2)}
	report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
		Path: "ptr.go", Status: "modified", Head: cleanResult("ptr.go", findings...),
	}}})

	Expect(report.Signals).To(HaveLen(2))
	for _, signal := range report.Signals {
		Expect(signal.RuleID).To(Equal("structure.pointer_return_density"))
		Expect(signal.Kind).To(Equal("pointer_return_density"))
		Expect(signal.Category).To(Equal(codesignal.Category("structure")))
		Expect(signal.Severity).To(Equal(codesignal.Severity("low")))
		Expect(signal.Confidence).To(Equal(codesignal.Confidence("medium")))
		Expect(signal.Provenance.FindingKind).To(Equal("pointer_return"))
	}
}

func body_registryTest_classifiesHeadOnlyDensitySignalsIntroducedWhenBa_117() {
	base := cleanResult("ctor.go", constructorFunc("NewA", 1))
	head := cleanResult("ctor.go", constructorFunc("NewA", 1), constructorFunc("NewB", 2))
	report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
		Path: "ctor.go", Status: "modified", Base: base, Head: head,
	}}})

	Expect(report.Signals).To(HaveLen(2))
	for _, signal := range report.Signals {
		Expect(signal.RuleID).To(Equal("structure.constructor_density"))
		Expect(signal.Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
	}
}

func body_registryTest_classifiesBaseOnlyDensitySignalsResolvedWithIncl_131() {
	base := cleanResult("ctor.go", constructorFunc("NewA", 1), constructorFunc("NewB", 2))
	head := cleanResult("ctor.go", constructorFunc("NewA", 1))

	defaultReport := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
		Path: "ctor.go", Status: "modified", Base: base, Head: head,
	}}})
	Expect(defaultReport.Signals).To(BeEmpty())

	includeResolvedReport := build(codesignal.Options{IncludeResolved: true}, codesignal.Input{Files: []codesignal.FileChange{{
		Path: "ctor.go", Status: "modified", Base: base, Head: head,
	}}})
	Expect(includeResolvedReport.Signals).To(HaveLen(2))
	for _, signal := range includeResolvedReport.Signals {
		Expect(signal.RuleID).To(Equal("structure.constructor_density"))
		Expect(signal.Lifecycle).To(Equal(codesignal.Lifecycle("resolved")))
	}
}

func body_registryTest_marksMatchingGatedDensitySignalsOnBothSidesExist_150() {
	base := cleanResult("ctor.go", constructorFunc("NewA", 1), constructorFunc("NewB", 2))
	head := cleanResult("ctor.go", constructorFunc("NewA", 10), constructorFunc("NewB", 20))
	report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
		Path: "ctor.go", Status: "modified", Base: base, Head: head,
	}}})

	Expect(report.Signals).To(HaveLen(2))
	for _, signal := range report.Signals {
		Expect(signal.Lifecycle).To(Equal(codesignal.Lifecycle("existing")))
	}
}

func body_registryTest_stillHardcodesConfidenceToMediumForTightCoupling_165() {
	const highConfidence = "high"
	tightCouplingHigh := semantics.Finding{Kind: "tight_coupling", Name: "NewThing", Confidence: highConfidence, Location: semantics.Location{StartRow: 1, EndRow: 1}}
	constructorFuncHighA := semantics.Finding{Kind: "constructor_func", Name: "NewA", Confidence: highConfidence, Location: semantics.Location{StartRow: 2, EndRow: 2}}
	constructorFuncHighB := semantics.Finding{Kind: "constructor_func", Name: "NewB", Confidence: highConfidence, Location: semantics.Location{StartRow: 3, EndRow: 3}}
	pointerReturnHighA := semantics.Finding{Kind: "pointer_return", Name: "NewC", Confidence: highConfidence, Location: semantics.Location{StartRow: 4, EndRow: 4}}
	pointerReturnHighB := semantics.Finding{Kind: "pointer_return", Name: "NewD", Confidence: highConfidence, Location: semantics.Location{StartRow: 5, EndRow: 5}}

	report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
		Path: "multi.go", Status: "modified", Head: cleanResult("multi.go",
			tightCouplingHigh, constructorFuncHighA, constructorFuncHighB, pointerReturnHighA, pointerReturnHighB,
		),
	}}})

	Expect(report.Signals).To(HaveLen(5))
	for _, signal := range report.Signals {
		Expect(signal.Confidence).To(Equal(codesignal.Confidence("medium")), "RuleID=%s should always be medium confidence, got %s", signal.RuleID, signal.Confidence)
	}
}

func body_registryTest_emitsASignalPerRecognizedFindingGatedIndependent_187() {
	findings := []semantics.Finding{
		mutation("Update", 1),
		tightCoupling("NewThing", 2),
		constructorFunc("NewA", 3),
		constructorFunc("NewB", 4),
		pointerReturn("NewC", 5),
	}
	report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
		Path: "multi.go", Status: "modified", Head: cleanResult("multi.go", findings...),
	}}})

	var kinds []string
	for _, signal := range report.Signals {
		kinds = append(kinds, signal.Kind)
	}
	Expect(kinds).To(ConsistOf(
		"hidden_input_mutation",
		"tight_constructor_init",
		"constructor_density",
		"constructor_density",
	))
}
