package main_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("eval-compare", func() {
	var stateRoot string

	BeforeEach(func() {
		var err error
		stateRoot, err = os.MkdirTemp("", "coach-eval-compare")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(os.RemoveAll, stateRoot)
	})

	When("two complete runs have the same comparable fields", func() {
		It("exits 0", func() {
			writeRunRecord(stateRoot, "run-a", completeRecord("run-a"))
			writeRunRecord(stateRoot, "run-b", completeRecord("run-b"))

			outcome := runCompare(stateRoot, "run-a", "run-b")

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		})
	})

	When("a run directory does not exist", func() {
		It("exits 2 and names the missing run", func() {
			writeRunRecord(stateRoot, "run-a", completeRecord("run-a"))

			outcome := runCompare(stateRoot, "run-a", "run-missing")

			Expect(outcome.exitCode).To(Equal(2))
			Expect(outcome.stderr).To(ContainSubstring("run-missing"))
		})
	})

	When("a run.json fails the committed schema", func() {
		It("exits 2 and names the offending key", func() {
			invalid := completeRecord("run-b")
			invalid["schema_version"] = "2"
			writeRunRecord(stateRoot, "run-a", completeRecord("run-a"))
			writeRunRecord(stateRoot, "run-b", invalid)

			outcome := runCompare(stateRoot, "run-a", "run-b")

			Expect(outcome.exitCode).To(Equal(2))
			Expect(outcome.stderr).To(ContainSubstring("schema_version"))
		})
	})

	DescribeTable("a run whose state is not complete", func(state string) {
		other := completeRecord("run-b")
		other["state"] = state
		writeRunRecord(stateRoot, "run-a", completeRecord("run-a"))
		writeRunRecord(stateRoot, "run-b", other)

		outcome := runCompare(stateRoot, "run-a", "run-b")

		Expect(outcome.exitCode).To(Equal(4))
		Expect(outcome.stderr).To(ContainSubstring(state))
	},
		Entry("is paused", "paused"),
		Entry("is incomplete", "incomplete"),
		Entry("is killed", "killed"),
		Entry("is invalid", "invalid"),
	)

	DescribeTable("a comparable field that differs", func(path string, value any) {
		changed := withPath(completeRecord("run-b"), path, value)
		writeRunRecord(stateRoot, "run-a", completeRecord("run-a"))
		writeRunRecord(stateRoot, "run-b", changed)

		outcome := runCompare(stateRoot, "run-a", "run-b")

		Expect(outcome.exitCode).To(Equal(4))
		Expect(outcome.stderr).To(ContainSubstring(path))
	},
		Entry("the solver name", "solver.name", "other-solver"),
		Entry("the solver model", "solver.model", "claude-other-model"),
		Entry("the CLI version", "solver.version", "9.9.9"),
		Entry("the isolation", "isolation", "docker"),
		Entry("the image", "image", "sha256:other-image"),
		Entry("the runtime", "runtime", "runsc"),
		Entry("the architecture", "architecture", "arm64"),
		Entry("a grader version", "grader_versions.typescript", "7.0.3"),
		Entry("the caps", "caps.trials", 361),
		Entry("the prompt", "prompt_hash", "sha256:other-prompt"),
		Entry("the oracle", "oracle_hash", "sha256:other-oracle"),
		Entry("the parameters", "parameters.k", 4),
	)

	DescribeTable("a worktree that is dirty on one side", func(dirtySide string) {
		a := completeRecord("run-a")
		b := completeRecord("run-b")
		if dirtySide == "run-a" {
			a["worktree_dirty"] = true
		} else {
			b["worktree_dirty"] = true
		}
		writeRunRecord(stateRoot, "run-a", a)
		writeRunRecord(stateRoot, "run-b", b)

		outcome := runCompare(stateRoot, "run-a", "run-b")

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		Expect(outcome.stderr).To(ContainSubstring(dirtySide))
	},
		Entry("is run-a", "run-a"),
		Entry("is run-b", "run-b"),
	)

	When("one run is missing and the other is not complete", func() {
		It("exits 2, because a missing input is checked before any state", func() {
			paused := completeRecord("run-a")
			paused["state"] = "paused"
			writeRunRecord(stateRoot, "run-a", paused)

			outcome := runCompare(stateRoot, "run-a", "run-missing")

			Expect(outcome.exitCode).To(Equal(2))
			Expect(outcome.stderr).To(ContainSubstring("run-missing"))
		})
	})

	When("a dirty run is also not complete", func() {
		It("exits 4 with only the state message on stderr, and no warning", func() {
			dirty := withPath(withPath(completeRecord("run-a"), "worktree_dirty", true), "state", "paused")
			writeRunRecord(stateRoot, "run-a", dirty)
			writeRunRecord(stateRoot, "run-b", completeRecord("run-b"))

			outcome := runCompare(stateRoot, "run-a", "run-b")

			Expect(outcome.exitCode).To(Equal(4), outcome.stderr)
			Expect(outcome.stdout).To(BeEmpty())
			Expect(outcome.stderr).To(ContainSubstring("run-a is paused"))
			Expect(outcome.stderr).NotTo(ContainSubstring("warning:"), "text output on exit 4 keeps the state message alone")
		})
	})
})
