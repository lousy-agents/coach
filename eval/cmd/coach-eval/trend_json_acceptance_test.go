package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func decodeJSONObject(stdout string) map[string]any {
	var document map[string]any
	ExpectWithOffset(1, json.Unmarshal([]byte(stdout), &document)).To(Succeed(), "stdout must be one JSON object: %s", stdout)
	return document
}

var _ = Describe("eval-compare trend --format json", func() {
	var root, ledger string

	BeforeEach(func() {
		var err error
		root, err = os.MkdirTemp("", "coach-eval-trend-json")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(os.RemoveAll, root)
		ledger = filepath.Join(root, "runs.jsonl")
	})

	When("a cell has adjacent comparable runs", func() {
		It("prints a JSON trend that validates against the trend output schema", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
				ledgerLine("pre_run", "2026-11-02", "cell-a", trendRecord("run-2", 0.75)),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger, "--format", "json")

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			expectMatchesSchema(trendOutputSchemaPath, []byte(outcome.stdout))
			document := decodeJSONObject(outcome.stdout)
			Expect(document["schema_version"]).To(Equal("1"))
			Expect(document["cell"]).To(Equal("cell-a"))
			Expect(document["metric"]).To(Equal("results.clean_success"))
			Expect(document["runs"]).To(BeEquivalentTo(2))
			Expect(document["breaks"]).To(Equal([]any{}), "no break means an empty array, not null")

			segments := document["segments"].([]any)
			Expect(segments).To(HaveLen(1))
			segment := segments[0].(map[string]any)
			Expect(segment["runs"]).To(Equal([]any{"run-1", "run-2"}))
			Expect(segment["change"]).To(BeNumerically("==", 0.25))
			first := segment["first"].(map[string]any)
			Expect(first["run_id"]).To(Equal("run-1"))
			Expect(first["value"]).To(BeNumerically("==", 0.5))
		})
	})

	When("a run differs in a comparable field", func() {
		It("lists the differing fields in the break and keeps the reason", func() {
			changed := withPath(trendRecord("run-3", 0.9), "solver.model", "claude-other-model")
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
				ledgerLine("pre_run", "2026-11-02", "cell-a", trendRecord("run-2", 0.75)),
				ledgerLine("pre_run", "2026-11-03", "cell-a", changed),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger, "--format", "json")

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			expectMatchesSchema(trendOutputSchemaPath, []byte(outcome.stdout))
			breaks := decodeJSONObject(outcome.stdout)["breaks"].([]any)
			Expect(breaks).To(HaveLen(1))
			entry := breaks[0].(map[string]any)
			Expect(entry["before"]).To(Equal("run-3"))
			Expect(entry["reason"]).To(Equal("solver.model differs"))
			Expect(entry["fields"]).To(Equal([]any{"solver.model"}))
		})
	})

	When("a run is not complete", func() {
		It("breaks with fields null, because the break is not a field mismatch", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
				ledgerLine("pre_run", "2026-11-02", "cell-a", trendRecord("run-2", 0.75)),
				ledgerLine("pre_run", "2026-11-03", "cell-a", withPath(trendRecord("run-3", 0.9), "state", "paused")),
				ledgerLine("pre_run", "2026-11-04", "cell-a", trendRecord("run-4", 0.8)),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger, "--format", "json")

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			expectMatchesSchema(trendOutputSchemaPath, []byte(outcome.stdout))
			breaks := decodeJSONObject(outcome.stdout)["breaks"].([]any)
			Expect(breaks).To(HaveLen(1))
			entry := breaks[0].(map[string]any)
			Expect(entry["reason"]).To(Equal("state paused"))
			Expect(entry).To(HaveKeyWithValue("fields", BeNil()), "a non-field break keeps the fields key, set to null")
		})
	})

	When("a run lacks the metric", func() {
		It("reports its value and the change of a one-run segment as null", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
				ledgerLine("pre_run", "2026-11-02", "cell-a", trendRecord("run-2", 0.75)),
				ledgerLine("pre_run", "2026-11-03", "cell-a", completeRecord("run-3")),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger, "--format", "json")

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			expectMatchesSchema(trendOutputSchemaPath, []byte(outcome.stdout))
			segments := decodeJSONObject(outcome.stdout)["segments"].([]any)
			Expect(segments).To(HaveLen(2))
			second := segments[1].(map[string]any)
			Expect(second).To(HaveKeyWithValue("change", BeNil()))
			first := second["first"].(map[string]any)
			Expect(first).To(HaveKeyWithValue("value", BeNil()))
		})
	})

	When("the format is not text or json", func() {
		It("exits 2 and names the format", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger, "--format", "yaml")

			Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
			Expect(outcome.stderr).To(ContainSubstring("--format"))
		})
	})

	When("no format is given", func() {
		It("prints the text trend, unchanged", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
				ledgerLine("pre_run", "2026-11-02", "cell-a", trendRecord("run-2", 0.75)),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(outcome.stdout).To(HavePrefix("cell cell-a:"))
			Expect(json.Valid([]byte(outcome.stdout))).To(BeFalse(), "text output is not JSON")
		})
	})

	When("the cell has no comparable adjacent pair", func() {
		It("exits 4 and prints nothing to stdout, as the text form does", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger, "--format", "json")

			Expect(outcome.exitCode).To(Equal(4), outcome.stderr)
			Expect(outcome.stdout).To(BeEmpty())
		})
	})

	When("run through the eval-trend mise task with --format json", func() {
		It("prints the JSON trend through the task", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
				ledgerLine("pre_run", "2026-11-02", "cell-a", trendRecord("run-2", 0.75)),
			)

			outcome := runTrendThroughMise(root, "--cell", "cell-a", "--ledger", ledger, "--format", "json")

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			expectMatchesSchema(trendOutputSchemaPath, []byte(outcome.stdout))
		})
	})
})
