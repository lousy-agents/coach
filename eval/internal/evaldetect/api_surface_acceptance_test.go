package evaldetect_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evaldetect"
)

const surfaceRule = "api_surface.symbol_absent"

var _ = Describe("the api_surface check (design §4, G2: an api_surface symbol missing or unreachable)", func() {
	entry := evaldetect.Symbol{Dir: "internal/api/entry", Name: "Run"}

	check := func(files map[string]string, symbols ...evaldetect.Symbol) ([]evaldetect.Finding, error) {
		return evaldetect.APISurfaceAbsences(writeTree(GinkgoT().TempDir(), files), symbols)
	}

	It("reports nothing when each symbol is an exported function of its package (AC-30)", func() {
		findings, err := check(map[string]string{
			"internal/api/entry/entry.go": "package entry\n\nfunc Run() {}\n",
		}, entry)

		Expect(err).NotTo(HaveOccurred())
		Expect(findings).To(BeEmpty())
	})

	It("accepts an exported method of the name, because the surface names methods and functions alike (AC-30)", func() {
		findings, err := check(map[string]string{
			"internal/api/entry/entry.go": "package entry\n\ntype Service struct{}\n\nfunc (Service) Run() {}\n",
		}, entry)

		Expect(err).NotTo(HaveOccurred())
		Expect(findings).To(BeEmpty())
	})

	DescribeTable("reports the symbol as absent, naming the rule and the symbol",
		func(files map[string]string) {
			findings, err := check(files, entry)

			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(ConsistOf(evaldetect.Finding{
				RuleID: surfaceRule, Path: "internal/api/entry", Subject: surfaceRule + ":internal/api/entry.Run",
			}))
		},
		Entry("when the package has no such function (AC-30)", map[string]string{
			"internal/api/entry/entry.go": "package entry\n\nfunc Other() {}\n",
		}),
		Entry("when only a function whose name starts with the symbol's is declared (AC-30)", map[string]string{
			"internal/api/entry/entry.go": "package entry\n\nfunc Runner() {}\n",
		}),
		Entry("when only a function whose name ends with the symbol's is declared (AC-30)", map[string]string{
			"internal/api/entry/entry.go": "package entry\n\nfunc MustRun() {}\n",
		}),
		Entry("when the name is not exported (AC-30)", map[string]string{
			"internal/api/entry/entry.go": "package entry\n\nfunc run() {}\n",
		}),
		Entry("when the name is a variable and not a function (AC-30)", map[string]string{
			"internal/api/entry/entry.go": "package entry\n\nvar Run = func() {}\n",
		}),
		Entry("when the only declaration is in a test file (AC-30)", map[string]string{
			"internal/api/entry/entry.go":      "package entry\n",
			"internal/api/entry/entry_test.go": "package entry\n\nfunc Run() {}\n",
		}),
		Entry("when the package directory does not exist (AC-30)", map[string]string{
			"internal/api/other/other.go": "package other\n\nfunc Run() {}\n",
		}),
		Entry("when the function is declared in another package's directory (AC-30)", map[string]string{
			"internal/api/entry/entry.go": "package entry\n",
			"internal/service/svc.go":     "package service\n\nfunc Run() {}\n",
		}),
	)

	It("never finds a symbol whose name is not exported, even when a function of that name is declared (AC-30)", func() {
		findings, err := check(map[string]string{
			"internal/api/entry/entry.go": "package entry\n\nfunc run() {}\n",
		}, evaldetect.Symbol{Dir: "internal/api/entry", Name: "run"})

		Expect(err).NotTo(HaveOccurred())
		Expect(findings).To(HaveLen(1))
	})

	It("reports each absent symbol of several, in a stable order (AC-30)", func() {
		findings, err := check(map[string]string{
			"internal/api/entry/entry.go": "package entry\n\nfunc Run() {}\n",
			"internal/service/svc.go":     "package service\n",
			"internal/store/store.go":     "package store\n",
		}, evaldetect.Symbol{Dir: "internal/store", Name: "Update"}, entry, evaldetect.Symbol{Dir: "internal/service", Name: "Update"})

		Expect(err).NotTo(HaveOccurred())
		Expect(findings).To(HaveLen(2))
		Expect(findings[0].Subject).To(Equal(surfaceRule + ":internal/service.Update"))
		Expect(findings[1].Subject).To(Equal(surfaceRule + ":internal/store.Update"))
	})

	It("refuses a Go file it cannot parse, instead of reporting a symbol absent from a package it could not read (AC-30)", func() {
		_, err := check(map[string]string{
			"internal/api/entry/entry.go": "package entry\n\nfunc Run( {\n",
		}, entry)

		Expect(err).To(MatchError(ContainSubstring("entry.go")))
	})
})
