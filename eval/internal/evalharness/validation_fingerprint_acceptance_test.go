package evalharness_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalharness"
)

var _ = Describe("the validation fingerprint a cached program is keyed by (design §6, \"Caching\")", func() {
	steps := []string{"native", "g2", "mutation"}

	It("is the same for the same ordered steps and version (AC-32)", func() {
		Expect(evalharness.ValidationFingerprint(steps, "1")).To(Equal(evalharness.ValidationFingerprint([]string{"native", "g2", "mutation"}, "1")))
	})

	DescribeTable("changes when only the step list changes, so a program cached under an older list is regenerated (AC-32)",
		func(other []string) {
			Expect(evalharness.ValidationFingerprint(other, "1")).NotTo(Equal(evalharness.ValidationFingerprint(steps, "1")))
		},
		Entry("a step added", []string{"native", "g2", "g1_native", "mutation"}),
		Entry("a step removed", []string{"native", "mutation"}),
		Entry("the steps in another order", []string{"native", "mutation", "g2"}),
		Entry("a step renamed", []string{"native", "g2x", "mutation"}),
	)

	It("changes when only the version changes, for a change inside a step (AC-32)", func() {
		Expect(evalharness.ValidationFingerprint(steps, "2")).NotTo(Equal(evalharness.ValidationFingerprint(steps, "1")))
	})

	It("is past version 1, because the api_surface and required-edge checks arrived inside the g2 step after it (AC-32)", func() {
		Expect(evalharness.ValidationVersion).NotTo(Equal("1"))
		Expect(evalharness.ValidationFingerprint(steps, evalharness.ValidationVersion)).NotTo(Equal(evalharness.ValidationFingerprint(steps, "1")))
	})

	It("keeps two lists apart that run together to the same text (AC-32)", func() {
		Expect(evalharness.ValidationFingerprint([]string{"ab", "c"}, "1")).NotTo(Equal(evalharness.ValidationFingerprint([]string{"a", "bc"}, "1")))
	})
})
