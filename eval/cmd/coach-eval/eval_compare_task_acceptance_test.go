package main_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("the eval-compare mise task", func() {
	var stateRoot string

	BeforeEach(func() {
		var err error
		stateRoot, err = os.MkdirTemp("", "coach-eval-compare-task")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(os.RemoveAll, stateRoot)
	})

	When("run with two run ids", func() {
		It("reaches the compare command with those run ids", func() {
			writeRunRecord(stateRoot, "run-a", completeRecord("run-a"))
			writeRunRecord(stateRoot, "run-b", completeRecord("run-b"))

			outcome := runCompareThroughMise(stateRoot, "run-a", "run-b")

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(outcome.stdout).To(ContainSubstring("comparable: run-a and run-b"))
		})
	})
})
