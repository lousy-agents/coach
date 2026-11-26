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

	When("a run is missing", func() {
		It("exits 2 through the task and names the missing run", func() {
			writeRunRecord(stateRoot, "run-a", completeRecord("run-a"))

			outcome := runCompareThroughMise(stateRoot, "run-a", "run-missing")

			Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
			Expect(outcome.stderr).To(ContainSubstring("run-missing"))
		})
	})

	When("two runs are not comparable", func() {
		It("exits 4 through the task and names the differing field", func() {
			changed := completeRecord("run-b")
			setPath(changed, "solver.model", "claude-other-model")
			writeRunRecord(stateRoot, "run-a", completeRecord("run-a"))
			writeRunRecord(stateRoot, "run-b", changed)

			outcome := runCompareThroughMise(stateRoot, "run-a", "run-b")

			Expect(outcome.exitCode).To(Equal(4), outcome.stderr)
			Expect(outcome.stderr).To(ContainSubstring("solver.model"))
		})
	})
})
