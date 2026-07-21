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
		})

		It("classifies base-only density signals resolved (with IncludeResolved) when head drops below its own gate", func() {
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
		})

		It("marks matching gated density signals on both sides existing", func() {
			base := cleanResult("ctor.go", constructorFunc("NewA", 1), constructorFunc("NewB", 2))
			head := cleanResult("ctor.go", constructorFunc("NewA", 10), constructorFunc("NewB", 20))
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "ctor.go", Status: "modified", Base: base, Head: head,
			}}})

			Expect(report.Signals).To(HaveLen(2))
			for _, signal := range report.Signals {
				Expect(signal.Lifecycle).To(Equal(codesignal.Lifecycle("existing")))
			}
		})
	})

	When("multiple rule kinds appear together in one file", func() {
		It("emits a signal per recognized finding, gated independently by kind", func() {
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
		})
	})
})
