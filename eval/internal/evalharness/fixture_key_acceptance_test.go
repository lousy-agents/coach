package evalharness_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalharness"
)

var _ = Describe("the fixture cache key", func() {
	base := func() string { return evalharness.FixtureKey("generator", "coach-hash", "v1.3.0", "validation") }

	It("is the same for the same inputs (AC-23)", func() {
		Expect(base()).To(Equal(evalharness.FixtureKey("generator", "coach-hash", "v1.3.0", "validation")))
	})

	DescribeTable("changes when any one input changes (AC-23)",
		func(generator, coach, detector, validation string) {
			Expect(evalharness.FixtureKey(generator, coach, detector, validation)).NotTo(Equal(base()))
		},
		Entry("the generator key", "another generator", "coach-hash", "v1.3.0", "validation"),
		Entry("the coach binary", "generator", "another coach", "v1.3.0", "validation"),
		Entry("the detector version", "generator", "coach-hash", "v1.4.0", "validation"),
		Entry("the validation fingerprint", "generator", "coach-hash", "v1.3.0", "another validation"),
	)

	It("keeps a run with no coach binary apart from a coach whose hash is the word none (AC-23)", func() {
		Expect(evalharness.FixtureKey("generator", "", "v1.3.0", "validation")).NotTo(Equal(evalharness.FixtureKey("generator", "none", "v1.3.0", "validation")))
	})
})
