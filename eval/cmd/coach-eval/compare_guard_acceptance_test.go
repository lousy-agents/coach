package main_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("the eval-compare guard rules", func() {
	var stateRoot string

	BeforeEach(func() {
		var err error
		stateRoot, err = os.MkdirTemp("", "coach-eval-compare-guard")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(os.RemoveAll, stateRoot)
	})

	DescribeTable("a command that is not given exactly two run ids", func(runIDs []string) {
		writeRunRecord(stateRoot, "run-a", completeRecord("run-a"))
		writeRunRecord(stateRoot, "run-b", completeRecord("run-b"))
		writeRunRecord(stateRoot, "run-c", completeRecord("run-c"))

		outcome := runCompare(stateRoot, runIDs...)

		Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
		Expect(outcome.stderr).To(ContainSubstring("expected exactly two run ids"))
	},
		Entry("is given no run id", []string{}),
		Entry("is given one run id", []string{"run-a"}),
		Entry("is given three run ids", []string{"run-a", "run-b", "run-c"}),
	)

	DescribeTable("a first run whose state is not complete", func(state string) {
		first := completeRecord("run-a")
		first["state"] = state
		writeRunRecord(stateRoot, "run-a", first)
		writeRunRecord(stateRoot, "run-b", completeRecord("run-b"))

		outcome := runCompare(stateRoot, "run-a", "run-b")

		Expect(outcome.exitCode).To(Equal(4), outcome.stderr)
		Expect(outcome.stderr).To(ContainSubstring("run-a is " + state))
	},
		Entry("is paused", "paused"),
		Entry("is incomplete", "incomplete"),
		Entry("is killed", "killed"),
		Entry("is invalid", "invalid"),
	)

	DescribeTable("a field outside the comparable set that differs", func(path string, value any) {
		changed := withPath(completeRecord("run-b"), path, value)
		writeRunRecord(stateRoot, "run-a", completeRecord("run-a"))
		writeRunRecord(stateRoot, "run-b", changed)

		outcome := runCompare(stateRoot, "run-a", "run-b")

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		Expect(outcome.stdout).To(ContainSubstring("comparable: run-a and run-b"))
	},
		Entry("the coach commit", "coach_sha", "fedcba9876543210fedcba9876543210fedcba98"),
		Entry("the protocol", "protocol_hash", "sha256:other-protocol"),
		Entry("the results", "results.clean_success", 3),
		Entry("the usage", "usage.hours", 9.5),
	)

	DescribeTable("a grader version that only one run records", func(side string) {
		a := completeRecord("run-a")
		b := completeRecord("run-b")
		recordingRun := b
		if side == "run-a" {
			recordingRun = a
		}
		recordingRun["grader_versions"].(record)["gocognit-extra"] = "v0.1.0"
		writeRunRecord(stateRoot, "run-a", a)
		writeRunRecord(stateRoot, "run-b", b)

		outcome := runCompare(stateRoot, "run-a", "run-b")

		Expect(outcome.exitCode).To(Equal(4), outcome.stderr)
		Expect(outcome.stderr).To(ContainSubstring("grader_versions.gocognit-extra"))
	},
		Entry("is recorded by run-a", "run-a"),
		Entry("is recorded by run-b", "run-b"),
	)
})
