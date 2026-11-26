package evaldetect_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evaldetect"
)

const pinnedGocognit = "v1.3.0"

var _ = Describe("the gocognit adapter", func() {
	When("functions sit just below and at Coach's threshold of 15", func() {
		It("reports the function at 15 under complexity.cognitive_complexity and not the one at 14", func() {
			root := writeTree(GinkgoT().TempDir(), map[string]string{
				"go.mod":     "module example.com/fx\n\ngo 1.26.0\n",
				"pkg/c/c.go": "package c\n\n" + ifChain("Fourteen", 14) + "\n" + ifChain("Fifteen", 15),
			})

			findings, err := evaldetect.Gocognit{Binary: gocognitBinary(), Version: pinnedGocognit}.Findings(root)
			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(ConsistOf(evaldetect.Finding{
				RuleID:  "complexity.cognitive_complexity",
				Path:    "pkg/c/c.go",
				Subject: "Fifteen",
				Line:    3 + 14*3 + 4 + 1,
			}))
		})
	})

	When("the installed gocognit is not the pinned version", func() {
		It("refuses, naming both versions", func() {
			err := evaldetect.Gocognit{Binary: gocognitBinary(), Version: "v1.2.0"}.CheckVersion()

			Expect(err).To(MatchError(And(ContainSubstring("v1.2.0"), ContainSubstring(pinnedGocognit))))
		})
	})

	When("the installed gocognit is the pinned version", func() {
		It("accepts it", func() {
			Expect(evaldetect.Gocognit{Binary: gocognitBinary(), Version: pinnedGocognit}.CheckVersion()).To(Succeed())
		})
	})
})
