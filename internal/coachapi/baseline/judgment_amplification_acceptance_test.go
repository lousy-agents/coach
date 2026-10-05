package baseline_test

import (
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("repo_baseline_scan local-model judgment amplification harness (Task 6)", func() {
	When("a lousy-iam-shaped fixture has 22+7+6+4+3 hidden_mutation signals and the gateway is slow", func() {
		It("completes under a short judgment wall with ≥1 source=agent finding via packed+capped judgment where pure 1:1 cannot finish in time", func() {
			expectPackedCappedJudgmentFinishesUnderShortWall()
		})
	})
})
