package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	detectorVersionVariable   = "COACH_EVAL_DETECTOR_VERSION"
	validationStepsVariable   = "COACH_EVAL_VALIDATION_STEPS"
	validationVersionVariable = "COACH_EVAL_VALIDATION_VERSION"
)

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
	DescribeTable("regenerates a program cached under another validation step list or another validation version, and serves it again under the first (AC-32)",
		func(changed string) {
			cacheHome := GinkgoT().TempDir()
			plain := noCoach()
			other := noCoach(changed)

			first := runGenerateBinary(faultBinary, cacheHome, plain, args...)
			second := runGenerateBinary(faultBinary, cacheHome, plain, args...)
			third := runGenerateBinary(faultBinary, cacheHome, other, args...)
			fourth := runGenerateBinary(faultBinary, cacheHome, plain, args...)

			Expect([]int{first.exitCode, second.exitCode, third.exitCode, fourth.exitCode}).To(Equal([]int{0, 0, 0, 0}))
			Expect(decodeGenerated(first.stdout).Cache).To(Equal("miss"))
			Expect(decodeGenerated(second.stdout).Cache).To(Equal("hit"))
			Expect(decodeGenerated(third.stdout).Cache).To(Equal("miss"), "an entry validated under another step list or version is not served")
			Expect(decodeGenerated(fourth.stdout).Cache).To(Equal("hit"), "the first entry is still there")
		},
		Entry("only the step list changes", validationStepsVariable+"=native,delete_baseline,g2"),
		Entry("only the version changes", validationVersionVariable+"=another"),
	)

	It("is not moved by either override in the release build (AC-32)", func() {
		cacheHome := GinkgoT().TempDir()

		first := runGenerate(cacheHome, noCoach(), args...)
		second := runGenerate(cacheHome, noCoach(validationStepsVariable+"=native", validationVersionVariable+"=another"), args...)

		Expect(decodeGenerated(first.stdout).Cache).To(Equal("miss"))
		Expect(decodeGenerated(second.stdout).Cache).To(Equal("hit"))
	})
})
