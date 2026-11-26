package evalcensus_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

var _ = Describe("the order a seeded draw sorts findings by", func() {
	base := evalcensus.FindingKey{RuleID: scopedRule, Path: "src/a.go", Subject: "f", Fingerprint: "fp1", StartRow: 3, EndRow: 9}

	When("two findings differ only in their path", func() {
		It("orders them by path", func() {
			other := base
			other.Path = "src/b.go"

			Expect(evalcensus.CompareFindingKeys(base, other)).To(BeNumerically("<", 0))
			Expect(evalcensus.CompareFindingKeys(other, base)).To(BeNumerically(">", 0))
		})
	})

	When("two findings differ only in their start row", func() {
		It("orders them by start row", func() {
			other := base
			other.StartRow = 4

			Expect(evalcensus.CompareFindingKeys(base, other)).To(BeNumerically("<", 0))
		})
	})

	When("two findings differ only in their end row", func() {
		It("orders them by end row", func() {
			other := base
			other.EndRow = 10

			Expect(evalcensus.CompareFindingKeys(base, other)).To(BeNumerically("<", 0))
		})
	})

	When("two findings differ only in their subject", func() {
		It("orders them by subject", func() {
			other := base
			other.Subject = "g"

			Expect(evalcensus.CompareFindingKeys(base, other)).To(BeNumerically("<", 0))
		})
	})

	When("two findings differ only in their fingerprint", func() {
		It("orders them by fingerprint", func() {
			other := base
			other.Fingerprint = "fp2"

			Expect(evalcensus.CompareFindingKeys(base, other)).To(BeNumerically("<", 0))
		})
	})
})
