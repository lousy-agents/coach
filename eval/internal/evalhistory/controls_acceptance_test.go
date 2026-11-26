package evalhistory_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalhistory"
)

// unitOfLines is a unit of n lines starting at line start of path.
func unitOfLines(path string, start, n int) evalhistory.Unit {
	return evalhistory.Unit{Path: path, Start: start, End: start + n - 1}
}

var _ = Describe("drawing size-matched controls", func() {
	pool := []evalhistory.Unit{
		unitOfLines("a.go", 1, 10),  // 0: flagged, bin 3
		unitOfLines("a.go", 20, 12), // 1: bin 3
		unitOfLines("a.go", 40, 9),  // 2: bin 3
		unitOfLines("b.go", 1, 15),  // 3: bin 3
		unitOfLines("b.go", 20, 8),  // 4: bin 3
		unitOfLines("b.go", 40, 40), // 5: bin 5
		unitOfLines("c.go", 1, 3),   // 6: bin 1
	}

	It("draws k unflagged units from each flagged unit's size bin, without replacement", func() {
		draw := evalhistory.DrawControls(pool, []int{0}, 3, 7)

		Expect(draw.Controls).To(HaveLen(3))
		Expect(draw.Controls).To(SatisfyAll(Not(ContainElement(0)), Not(ContainElement(5)), Not(ContainElement(6))))
		Expect(draw.Shortfall).To(BeZero())
	})

	It("draws exactly k controls per flagged unit when the bin has room", func() {
		Expect(evalhistory.DrawControls(pool, []int{0}, 1, 7).Controls).To(HaveLen(1))
		Expect(evalhistory.DrawControls(pool, []int{0}, 2, 7).Controls).To(HaveLen(2))
	})

	It("draws the same controls for the same seed, and may differ for another", func() {
		first := evalhistory.DrawControls(pool, []int{0}, 2, 7)
		again := evalhistory.DrawControls(pool, []int{0}, 2, 7)

		Expect(again).To(Equal(first))
		differs := false
		for seed := int64(1); seed < 40 && !differs; seed++ {
			differs = !equalInts(evalhistory.DrawControls(pool, []int{0}, 2, seed).Controls, first.Controls)
		}
		Expect(differs).To(BeTrue(), "the seed must steer the draw")
	})

	It("records a shortfall when a bin has too few unflagged units", func() {
		draw := evalhistory.DrawControls(pool, []int{5}, 3, 7)

		Expect(draw.Controls).To(BeEmpty())
		Expect(draw.Shortfall).To(Equal(3))
	})

	It("never uses a control twice or draws another flagged unit", func() {
		draw := evalhistory.DrawControls(pool, []int{0, 3}, 2, 7)

		Expect(draw.Controls).To(ConsistOf(1, 2, 4), "the bin holds three unflagged units for four draws")
		Expect(draw.Shortfall).To(Equal(1))
		Expect(draw.Controls).To(SatisfyAll(Not(ContainElement(0)), Not(ContainElement(3))))
		seen := map[int]bool{}
		for _, control := range draw.Controls {
			Expect(seen[control]).To(BeFalse(), "control %d drawn twice", control)
			seen[control] = true
		}
	})
})

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

var _ = Describe("the flagged-vs-unflagged risk ratio", func() {
	It("divides the flagged rate by the control rate, with a Katz log 95% interval", func() {
		association := evalhistory.RiskRatio(6, 10, 3, 30)

		Expect(*association.RiskRatio).To(BeNumerically("~", 6.0, 1e-9))
		Expect(*association.Low).To(BeNumerically("~", 1.8311, 1e-3))
		Expect(*association.High).To(BeNumerically("~", 19.6598, 1e-3))
		Expect(association.Reason).To(BeEmpty())
	})

	DescribeTable("a ratio that cannot be estimated is null, with the reason",
		func(a, n1, b, n0 int, reason string) {
			association := evalhistory.RiskRatio(a, n1, b, n0)

			Expect(association.RiskRatio).To(BeNil())
			Expect(association.Low).To(BeNil())
			Expect(association.High).To(BeNil())
			Expect(association.Reason).To(ContainSubstring(reason))
		},
		Entry("no flagged unit", 0, 0, 3, 30, "no flagged unit"),
		Entry("no control", 6, 10, 0, 0, "no control"),
		Entry("no flagged unit involved", 0, 10, 3, 30, "zero cell"),
		Entry("no control involved", 6, 10, 0, 30, "zero cell"),
	)
})
