package render

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// A bound of git that stopped the read of HEAD's copy is named in the warning; the JSON cause stays head_unreadable and
// the frozen scope.policy gains no field (#168 A2b).
var _ = Describe("ReportText and ReportJSON for a policy whose copy at HEAD was stopped by a bound of git (#168 A2b)", func() {
	boundedBy := func(bound codesignal.HeadBound) *codesignal.PolicyScope {
		policy := scopePolicy(false, codesignal.PolicyCauseHeadUnreadable)
		policy.HeadBound = bound
		return policy
	}

	DescribeTable("the warning names the bound",
		func(bound codesignal.HeadBound, words string) {
			for _, layout := range policyLayouts {
				text := ReportText(policyReport(layout, boundedBy(bound)))

				after := linesAfterSummary(layout, text, 2)
				Expect(after[0]).To(HavePrefix(policyTestPrefix))
				Expect(after[1]).To(MatchRegexp(`^WARNING: .*HEAD.*could not be read within `+words), "%s text:\n%s", layout.name, text)
				Expect(after[1]).NotTo(ContainSubstring("differs from HEAD's"))
			}
		},
		Entry("git's time bound", codesignal.HeadBoundTime, `git's time bound`),
		Entry("git's stderr bound", codesignal.HeadBoundStderr, `git's stderr bound`),
	)

	It("names no bound for a copy that was over the size budget or that git could not read", func() {
		text := ReportText(policyReport(policyLayouts[0], boundedBy(codesignal.HeadBoundNone)))

		after := linesAfterSummary(policyLayouts[0], text, 2)
		Expect(after[1]).To(MatchRegexp(`^WARNING: .*HEAD.*could not be read[;.]`))
		Expect(after[1]).NotTo(ContainSubstring("bound"))
	})

	DescribeTable("serializes the cause head_unreadable and no field for the bound",
		func(bound codesignal.HeadBound) {
			data, err := ReportJSON(policyReport(policyLayouts[0], boundedBy(bound)))

			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring(`"matches_head":false,"cause":"head_unreadable"}`))
		},
		Entry("git's time bound", codesignal.HeadBoundTime),
		Entry("git's stderr bound", codesignal.HeadBoundStderr),
	)
})
