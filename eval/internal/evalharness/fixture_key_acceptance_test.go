package evalharness_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalharness"
)

var _ = Describe("the fixture cache key", func() {
	base := func() string {
		return evalharness.FixtureKey("generator", "coach-hash", "v1.3.0", "validation", "checked")
	}

	It("is the same for the same inputs (AC-23)", func() {
		Expect(base()).To(Equal(evalharness.FixtureKey("generator", "coach-hash", "v1.3.0", "validation", "checked")))
	})

	DescribeTable("changes when any one input changes (AC-23, #906)",
		func(generator, coach, detector, validation, gate string) {
			Expect(evalharness.FixtureKey(generator, coach, detector, validation, gate)).NotTo(Equal(base()))
		},
		Entry("the generator key", "another generator", "coach-hash", "v1.3.0", "validation", "checked"),
		Entry("the coach binary", "generator", "another coach", "v1.3.0", "validation", "checked"),
		Entry("the detector version", "generator", "coach-hash", "v1.4.0", "validation", "checked"),
		Entry("the validation fingerprint", "generator", "coach-hash", "v1.3.0", "another validation", "checked"),
		Entry("the mutation gate, which a skipped run keys its entries by", "generator", "coach-hash", "v1.3.0", "validation", "skipped"),
	)

	It("keeps a run with no coach binary apart from a coach whose hash is the word none (AC-23)", func() {
		Expect(evalharness.FixtureKey("generator", "", "v1.3.0", "validation", "checked")).NotTo(Equal(evalharness.FixtureKey("generator", "none", "v1.3.0", "validation", "checked")))
	})
})
