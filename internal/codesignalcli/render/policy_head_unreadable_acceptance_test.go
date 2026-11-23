package render

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// head_unreadable is the closed cause for a policy on disk that was used when HEAD's copy could not be read to compare (#168 A2b).
var _ = Describe("ReportText and ReportJSON for a policy whose copy at HEAD could not be read (#168 A2b)", func() {
	unreadable := func() *codesignal.PolicyScope {
		return scopePolicy(false, codesignal.PolicyCause("head_unreadable"))
	}

	It("serializes the cause head_unreadable beside path, digest and matches_head false", func() {
		data, err := ReportJSON(policyReport(policyLayouts[0], unreadable()))

		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring(`"matches_head":false,"cause":"head_unreadable"}`))
	})

	for _, layout := range policyLayouts {
		It("says in the "+layout.name+" text that HEAD's copy could not be read, and not that the content differs", func() {
			text := ReportText(policyReport(layout, unreadable()))

			after := linesAfterSummary(layout, text, 2)
			Expect(after[0]).To(HavePrefix(policyTestPrefix))
			Expect(after[0]).NotTo(HaveSuffix("same as HEAD."))
			Expect(after[1]).To(MatchRegexp(`^WARNING: .*HEAD.*could not be read`), "text:\n%s", text)
			Expect(after[1]).NotTo(Equal(policyDiffersWarning))
		})
	}

	It("qualifies a result with no findings, naming HEAD", func() {
		text := ReportText(policyReport(policyLayouts[0], unreadable()))

		Expect(text).To(MatchRegexp(`(?m)^No active CodeSignal findings, but .*HEAD.*\.$`))
		Expect(text).NotTo(ContainSubstring("\n" + policyUnqualifiedClear + "\n"))
	})
})
