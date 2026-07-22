package codesignal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func resultWithImports(path string, language semantics.Language, imports ...semantics.ImportFeature) *semantics.Result {
	return &semantics.Result{Path: path, Language: language, ParseStatus: "ok", Imports: imports}
}

var _ = Describe("coupling.deep_relative_import", func() {
	When("a TS import climbs three or more directories", func() {
		It("emits exactly one signal with the locked field shape", func() {
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "src/a.ts", Status: "modified",
				Head: resultWithImports("src/a.ts", semantics.LanguageTypeScript, semantics.ImportFeature{
					Path:     "../../../foo",
					Location: semantics.Location{StartRow: 1, EndRow: 1},
				}),
			}}})

			Expect(report.Signals).To(HaveLen(1))
			signal := report.Signals[0]
			Expect(signal.RuleID).To(Equal("coupling.deep_relative_import"))
			Expect(signal.RuleVersion).To(Equal("1"))
			Expect(signal.Kind).To(Equal("deep_relative_import"))
			Expect(signal.Category).To(Equal(codesignal.Category("coupling")))
			Expect(signal.Severity).To(Equal(codesignal.Severity("medium")))
			Expect(signal.Confidence).To(Equal(codesignal.Confidence("medium")))
			Expect(signal.Subject).To(Equal("../../../foo"))
			Expect(signal.Evidence).To(Equal("../../../foo"))
			Expect(signal.Location).To(Equal(semantics.Location{StartRow: 1, EndRow: 1}))
			Expect(signal.WhyItMatters).NotTo(BeEmpty())
			Expect(signal.Recommendation).NotTo(BeEmpty())
			Expect(signal.Provenance).To(Equal(codesignal.Provenance{Producer: "codesignal"}))
		})
	})

	When("all relative imports climb at most two directories", func() {
		It("emits no deep_relative_import signal", func() {
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "src/a.ts", Status: "modified",
				Head: resultWithImports("src/a.ts", semantics.LanguageTypeScript,
					semantics.ImportFeature{Path: "../../foo"},
					semantics.ImportFeature{Path: "./foo/bar/baz"},
				),
			}}})
			Expect(report.Signals).To(BeEmpty())
		})
	})

	When("a relative import mixes \".\" and \"..\" segments", func() {
		It("counts only the \"..\" segments toward depth", func() {
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "src/a.ts", Status: "modified",
				Head: resultWithImports("src/a.ts", semantics.LanguageTypeScript,
					semantics.ImportFeature{Path: ".././../../x"},
				),
			}}})
			Expect(report.Signals).To(HaveLen(1))
			Expect(report.Signals[0].Subject).To(Equal(".././../../x"))
		})
	})

	When("the import is an absolute/package import with many segments", func() {
		It("emits no deep_relative_import signal", func() {
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "src/a.tsx", Status: "modified",
				Head: resultWithImports("src/a.tsx", semantics.LanguageTSX,
					semantics.ImportFeature{Path: "some/deeply/nested/package/path"},
				),
			}}})
			Expect(report.Signals).To(BeEmpty())
		})
	})

	When("the file's language is Go", func() {
		It("emits no deep_relative_import signal even though the path string would match", func() {
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "a.go", Status: "modified",
				Head: resultWithImports("a.go", semantics.LanguageGo,
					semantics.ImportFeature{Path: "../../../foo"},
				),
			}}})
			Expect(report.Signals).To(BeEmpty())
		})
	})

	Describe("lifecycle classification", func() {
		It("marks a deep relative import present on both sides existing", func() {
			base := resultWithImports("src/a.ts", semantics.LanguageTypeScript, semantics.ImportFeature{Path: "../../../foo"})
			head := resultWithImports("src/a.ts", semantics.LanguageTypeScript, semantics.ImportFeature{Path: "../../../foo"})
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "src/a.ts", Status: "modified", Base: base, Head: head,
			}}})
			Expect(report.Signals).To(HaveLen(1))
			Expect(report.Signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("existing")))
		})

		It("marks a deep relative import only on head introduced", func() {
			base := resultWithImports("src/a.ts", semantics.LanguageTypeScript)
			head := resultWithImports("src/a.ts", semantics.LanguageTypeScript, semantics.ImportFeature{Path: "../../../foo"})
			report := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "src/a.ts", Status: "modified", Base: base, Head: head,
			}}})
			Expect(report.Signals).To(HaveLen(1))
			Expect(report.Signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("introduced")))
		})

		It("marks a deep relative import only on base resolved (with IncludeResolved)", func() {
			base := resultWithImports("src/a.ts", semantics.LanguageTypeScript, semantics.ImportFeature{Path: "../../../foo"})
			head := resultWithImports("src/a.ts", semantics.LanguageTypeScript)

			defaultReport := build(codesignal.Options{}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "src/a.ts", Status: "modified", Base: base, Head: head,
			}}})
			Expect(defaultReport.Signals).To(BeEmpty())

			includeResolvedReport := build(codesignal.Options{IncludeResolved: true}, codesignal.Input{Files: []codesignal.FileChange{{
				Path: "src/a.ts", Status: "modified", Base: base, Head: head,
			}}})
			Expect(includeResolvedReport.Signals).To(HaveLen(1))
			Expect(includeResolvedReport.Signals[0].RuleID).To(Equal("coupling.deep_relative_import"))
			Expect(includeResolvedReport.Signals[0].Lifecycle).To(Equal(codesignal.Lifecycle("resolved")))
		})
	})
})
