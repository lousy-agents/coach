package main_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("eval-compare compare --format json", func() {
	var stateRoot string

	BeforeEach(func() {
		var err error
		stateRoot, err = os.MkdirTemp("", "coach-eval-compare-json")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(os.RemoveAll, stateRoot)
	})

	When("two complete runs are comparable", func() {
		It("prints a comparable verdict that validates against the compare output schema", func() {
			writeRunRecord(stateRoot, "run-a", completeRecord("run-a"))
			writeRunRecord(stateRoot, "run-b", completeRecord("run-b"))

			outcome := runCompare(stateRoot, "--format", "json", "run-a", "run-b")

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(outcome.stderr).To(BeEmpty(), "JSON mode keeps warnings in the document")
			expectMatchesSchema(compareOutputSchemaPath, []byte(outcome.stdout))
			document := decodeJSONObject(outcome.stdout)
			Expect(document["schema_version"]).To(Equal("1"))
			Expect(document["comparable"]).To(BeTrue())
			Expect(document["runs"]).To(Equal([]any{"run-a", "run-b"}))
			Expect(document["differing_fields"]).To(Equal([]any{}))
			Expect(document["warnings"]).To(Equal([]any{}))
			Expect(document["not_complete_runs"]).To(Equal([]any{}))
		})
	})

	When("a comparable field differs", func() {
		It("reports comparable false and lists the differing fields on stdout", func() {
			changed := withPath(completeRecord("run-b"), "solver.model", "claude-other-model")
			writeRunRecord(stateRoot, "run-a", completeRecord("run-a"))
			writeRunRecord(stateRoot, "run-b", changed)

			outcome := runCompare(stateRoot, "--format", "json", "run-a", "run-b")

			Expect(outcome.exitCode).To(Equal(4), outcome.stderr)
			expectMatchesSchema(compareOutputSchemaPath, []byte(outcome.stdout))
			document := decodeJSONObject(outcome.stdout)
			Expect(document["comparable"]).To(BeFalse())
			Expect(document["differing_fields"]).To(Equal([]any{"solver.model"}))
		})
	})

	When("a run is not complete", func() {
		It("reports comparable false and names the run and its state", func() {
			paused := withPath(completeRecord("run-b"), "state", "paused")
			writeRunRecord(stateRoot, "run-a", completeRecord("run-a"))
			writeRunRecord(stateRoot, "run-b", paused)

			outcome := runCompare(stateRoot, "--format", "json", "run-a", "run-b")

			Expect(outcome.exitCode).To(Equal(4), outcome.stderr)
			expectMatchesSchema(compareOutputSchemaPath, []byte(outcome.stdout))
			document := decodeJSONObject(outcome.stdout)
			Expect(document["comparable"]).To(BeFalse())
			Expect(document["not_complete_runs"]).To(Equal([]any{
				map[string]any{"run_id": "run-b", "state": "paused"},
			}))
		})
	})

	When("a run is dirty on one side", func() {
		It("keeps the verdict comparable and names the dirty run in warnings", func() {
			dirty := withPath(completeRecord("run-a"), "worktree_dirty", true)
			writeRunRecord(stateRoot, "run-a", dirty)
			writeRunRecord(stateRoot, "run-b", completeRecord("run-b"))

			outcome := runCompare(stateRoot, "--format", "json", "run-a", "run-b")

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			expectMatchesSchema(compareOutputSchemaPath, []byte(outcome.stdout))
			warnings := decodeJSONObject(outcome.stdout)["warnings"].([]any)
			Expect(warnings).To(HaveLen(1))
			Expect(warnings[0]).To(ContainSubstring("run-a"))
		})
	})

	When("a run is missing", func() {
		It("exits 2 and prints nothing to stdout", func() {
			writeRunRecord(stateRoot, "run-a", completeRecord("run-a"))

			outcome := runCompare(stateRoot, "--format", "json", "run-a", "run-missing")

			Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
			Expect(outcome.stdout).To(BeEmpty())
			Expect(outcome.stderr).To(ContainSubstring("run-missing"))
		})
	})

	When("the format is not text or json", func() {
		It("exits 2 and names the format", func() {
			writeRunRecord(stateRoot, "run-a", completeRecord("run-a"))
			writeRunRecord(stateRoot, "run-b", completeRecord("run-b"))

			outcome := runCompare(stateRoot, "--format", "yaml", "run-a", "run-b")

			Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
			Expect(outcome.stderr).To(ContainSubstring("--format"))
		})
	})

	When("no format is given", func() {
		It("prints the text verdict, unchanged", func() {
			writeRunRecord(stateRoot, "run-a", completeRecord("run-a"))
			writeRunRecord(stateRoot, "run-b", completeRecord("run-b"))

			outcome := runCompare(stateRoot, "run-a", "run-b")

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(outcome.stdout).To(Equal("comparable: run-a and run-b\n"))
		})
	})
})
