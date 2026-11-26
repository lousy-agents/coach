package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("eval-compare and the profile a run used (#872 Task 4, AC-5.2, AC-5.3)", func() {
	It("accepts a run record that names its profile, and compares two runs of one profile", func() {
		state := GinkgoT().TempDir()
		writeRunRecord(state, "run-a", withPath(completeRecord("run-a"), "profile", "aaaa"))
		writeRunRecord(state, "run-b", withPath(completeRecord("run-b"), "profile", "aaaa"))

		outcome := runCompare(state, "run-a", "run-b")

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
	})

	DescribeTable("exits 4 naming profile when the two runs' profiles differ",
		func(a, b any) {
			state := GinkgoT().TempDir()
			first, second := completeRecord("run-a"), completeRecord("run-b")
			if a != nil {
				first = withPath(first, "profile", a)
			}
			if b != nil {
				second = withPath(second, "profile", b)
			}
			writeRunRecord(state, "run-a", first)
			writeRunRecord(state, "run-b", second)

			outcome := runCompare(state, "run-a", "run-b")

			Expect(outcome.exitCode).To(Equal(4), outcome.stderr)
			Expect(outcome.stderr).To(ContainSubstring("profile"))
		},
		Entry("two identities", "aaaa", "bbbb"),
		Entry("a profile against none", "aaaa", nil),
	)
})
