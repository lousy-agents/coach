package main_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("the committed run schema", func() {
	var stateRoot string

	BeforeEach(func() {
		var err error
		stateRoot, err = os.MkdirTemp("", "coach-eval-run-schema")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(os.RemoveAll, stateRoot)
	})

	DescribeTable("a run record that breaks the schema", func(breakRecord func(record), expectedText string) {
		broken := completeRecord("run-b")
		breakRecord(broken)
		writeRunRecord(stateRoot, "run-a", completeRecord("run-a"))
		writeRunRecord(stateRoot, "run-b", broken)

		outcome := runCompare(stateRoot, "run-a", "run-b")

		Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
		Expect(outcome.stderr).To(ContainSubstring(expectedText))
	},
		Entry("is missing a required key", func(r record) { delete(r, "isolation") }, "isolation"),
		Entry("has an unknown top-level key", func(r record) { r["extra"] = true }, "extra"),
		Entry("has a state outside the enum", func(r record) { r["state"] = "bogus" }, "state"),
		Entry("has worktree_dirty of the wrong type", func(r record) { r["worktree_dirty"] = "yes" }, "worktree_dirty"),
		Entry("has an unknown caps key", func(r record) { r["caps"].(record)["extra"] = 1 }, "extra"),
		Entry("is missing a solver key", func(r record) { delete(r["solver"].(record), "model") }, "model"),
	)
})
