package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const detectorVersionVariable = "COACH_EVAL_DETECTOR_VERSION"

var _ = Describe("the fixture cache's key and the pinned detector's version (AC-23)", Ordered, ContinueOnFailure, func() {
	var faultBinary string
	args := []string{"--cell", "1", "--seed", "3"}
	noCoach := func(extra ...string) []string {
		return append([]string{"PATH=" + pathWithOnlyGo(), "COACH_BIN="}, extra...)
	}

	BeforeAll(func() {
		faultBinary = buildCoachEvalWithFaultSeam()
	})

	It("misses the cache when only the pinned detector's version differs, and hits again under the first version", func() {
		cacheHome := GinkgoT().TempDir()

		first := runGenerateBinary(faultBinary, cacheHome, noCoach(detectorVersionVariable+"=v1.0.0"), args...)
		second := runGenerateBinary(faultBinary, cacheHome, noCoach(detectorVersionVariable+"=v1.0.0"), args...)
		third := runGenerateBinary(faultBinary, cacheHome, noCoach(detectorVersionVariable+"=v2.0.0"), args...)
		fourth := runGenerateBinary(faultBinary, cacheHome, noCoach(detectorVersionVariable+"=v1.0.0"), args...)

		Expect([]int{first.exitCode, second.exitCode, third.exitCode, fourth.exitCode}).To(Equal([]int{0, 0, 0, 0}))
		Expect(decodeGenerated(first.stdout).Cache).To(Equal("miss"))
		Expect(decodeGenerated(second.stdout).Cache).To(Equal("hit"), "the same version is the same key")
		Expect(decodeGenerated(third.stdout).Cache).To(Equal("miss"), "another detector version is another key")
		Expect(decodeGenerated(fourth.stdout).Cache).To(Equal("hit"), "the first version's record is still there")
	})

	It("is not moved by the override in the release build, which keys by the pinned version alone", func() {
		cacheHome := GinkgoT().TempDir()

		first := runGenerate(cacheHome, noCoach(detectorVersionVariable+"=v1.0.0"), args...)
		second := runGenerate(cacheHome, noCoach(detectorVersionVariable+"=v2.0.0"), args...)

		Expect(decodeGenerated(first.stdout).Cache).To(Equal("miss"))
		Expect(decodeGenerated(second.stdout).Cache).To(Equal("hit"))
	})
})
