package codesignal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func tightCoupling(name string, row uint) semantics.Finding {
	return semantics.Finding{Kind: "tight_coupling", Name: name, Location: semantics.Location{StartRow: row, EndRow: row}}
}

func constructorFunc(name string, row uint) semantics.Finding {
	return semantics.Finding{Kind: "constructor_func", Name: name, Location: semantics.Location{StartRow: row, EndRow: row}}
}

func pointerReturn(name string, row uint) semantics.Finding {
	return semantics.Finding{Kind: "pointer_return", Name: name, Location: semantics.Location{StartRow: row, EndRow: row}}
}

var _ = Describe("Rule registry dispatch", func() {
	When("a tight_coupling finding is present", func() {
		It("always emits a coupling.tight_constructor_init signal", func() {
			finding := tightCoupling("NewThing", 3)
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "ctor.go", Status: "modified", Head: cleanResult("ctor.go", finding),
			}}})

			Expect(report.Signals).To(HaveLen(1))
			signal := report.Signals[0]
			Expect(signal.RuleID).To(Equal("coupling.tight_constructor_init"))
			Expect(signal.RuleVersion).To(Equal("1"))
			Expect(signal.Kind).To(Equal("tight_constructor_init"))
			Expect(signal.Category).To(Equal(codesignal.Category("coupling")))
			Expect(signal.Severity).To(Equal(codesignal.Severity("medium")))
			Expect(signal.Confidence).To(Equal(codesignal.Confidence("medium")))
			Expect(signal.WhyItMatters).NotTo(BeEmpty())
			Expect(signal.Recommendation).NotTo(BeEmpty())
			Expect(signal.Provenance).To(Equal(codesignal.Provenance{Producer: "semantics", FindingKind: "tight_coupling"}))
			Expect(signal.Subject).To(Equal(finding.Name))
			Expect(signal.Location).To(Equal(finding.Location))
		})
	})

	When("a file has two or more constructor_func findings", func() {
		It("emits one structure.constructor_density signal per finding", func() {
			body_registryTest_emitsOneStructureConstructorDensitySignalPerFind_48()
		})
	})

	When("a file has exactly one constructor_func finding", func() {
		It("emits zero structure.constructor_density signals", func() {
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "ctor.go", Status: "modified", Head: cleanResult("ctor.go", constructorFunc("NewA", 1)),
			}}})
			Expect(report.Signals).To(BeEmpty())
		})
	})

	When("a file has two or more pointer_return findings", func() {
		It("emits one structure.pointer_return_density signal per finding", func() {
			body_registryTest_emitsOneStructurePointerReturnDensitySignalPerFi_79()
		})
	})

	When("a file has exactly one pointer_return finding", func() {
		It("emits zero structure.pointer_return_density signals", func() {
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "ptr.go", Status: "modified", Head: cleanResult("ptr.go", pointerReturn("NewA", 1)),
			}}})
			Expect(report.Signals).To(BeEmpty())
		})
	})

	When("an unrecognized finding kind is the only finding", func() {
		It("emits zero signals and zero diagnostics for that kind", func() {
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "x.go", Status: "modified", Head: cleanResult("x.go", semantics.Finding{Kind: "totally_unknown_kind", Name: "Mystery"}),
			}}})
			Expect(report.Signals).To(BeEmpty())
			Expect(report.Diagnostics).To(BeEmpty())
		})
	})

	Context("gating is independent per side", func() {
		It("classifies head-only density signals introduced when base was below its own gate", func() {
			body_registryTest_classifiesHeadOnlyDensitySignalsIntroducedWhenBa_117()
		})

		It("classifies base-only density signals resolved (with IncludeResolved) when head drops below its own gate", func() {
			body_registryTest_classifiesBaseOnlyDensitySignalsResolvedWithIncl_131()
		})

		It("marks matching gated density signals on both sides existing", func() {
			body_registryTest_marksMatchingGatedDensitySignalsOnBothSidesExist_150()
		})
	})

	When("the underlying finding reports a non-medium Confidence", func() {
		It("still hardcodes Confidence to medium for tight_coupling, constructor_density, and pointer_return_density", func() {
			body_registryTest_stillHardcodesConfidenceToMediumForTightCoupling_165()
		})
	})

	When("multiple rule kinds appear together in one file", func() {
		It("emits a signal per recognized finding, gated independently by kind", func() {
			body_registryTest_emitsASignalPerRecognizedFindingGatedIndependent_187()
		})
	})
})
