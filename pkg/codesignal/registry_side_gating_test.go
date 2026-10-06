package codesignal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("Rule registry dispatch", func() {
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
})
