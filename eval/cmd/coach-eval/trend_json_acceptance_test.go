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
		It("exits 4 and prints the JSON trend, its segments and breaks, with no change", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger, "--format", "json")

			Expect(outcome.exitCode).To(Equal(4), outcome.stderr)
			expectMatchesSchema(trendOutputSchemaPath, []byte(outcome.stdout))
			document := decodeJSONObject(outcome.stdout)
			segments := document["segments"].([]any)
			Expect(segments).To(HaveLen(1))
			Expect(segments[0].(map[string]any)).To(HaveKeyWithValue("change", BeNil()))
			Expect(outcome.stderr).To(ContainSubstring("no comparable adjacent pair"))
		})
	})

	When("no run of a cell is complete", func() {
		It("prints an empty segments array, not null, and exits 4", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", withPath(trendRecord("run-1", 0.5), "state", "paused")),
				ledgerLine("pre_run", "2026-11-02", "cell-a", withPath(trendRecord("run-2", 0.75), "state", "paused")),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger, "--format", "json")

			Expect(outcome.exitCode).To(Equal(4), outcome.stderr)
			expectMatchesSchema(trendOutputSchemaPath, []byte(outcome.stdout))
			document := decodeJSONObject(outcome.stdout)
			Expect(document["segments"]).To(Equal([]any{}), "no complete run leaves no segment, and an empty array stays an array")
			Expect(document["breaks"].([]any)).To(HaveLen(2), "each paused run is a break")
		})
	})

	When("a segment starts with a run that lacks the metric", func() {
		It("keeps the segment change null, because its first value is absent", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", completeRecord("run-1")),
				ledgerLine("pre_run", "2026-11-02", "cell-a", trendRecord("run-2", 0.5)),
				ledgerLine("pre_run", "2026-11-03", "cell-a", trendRecord("run-3", 0.75)),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger, "--format", "json")

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			expectMatchesSchema(trendOutputSchemaPath, []byte(outcome.stdout))
			segments := decodeJSONObject(outcome.stdout)["segments"].([]any)
			Expect(segments).To(HaveLen(1))
			segment := segments[0].(map[string]any)
			Expect(segment["runs"]).To(Equal([]any{"run-1", "run-2", "run-3"}))
			Expect(segment).To(HaveKeyWithValue("change", BeNil()))
			first := segment["first"].(map[string]any)
			Expect(first).To(HaveKeyWithValue("value", BeNil()))
		})
	})

	When("a metric value or change has more than six decimal places", func() {
		It("prints the value and the change rounded to six decimal places", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.1234567)),
				ledgerLine("pre_run", "2026-11-02", "cell-a", trendRecord("run-2", 0.7)),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger, "--format", "json")

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			expectMatchesSchema(trendOutputSchemaPath, []byte(outcome.stdout))
			segment := decodeJSONObject(outcome.stdout)["segments"].([]any)[0].(map[string]any)
			first := segment["first"].(map[string]any)
			Expect(first["value"]).To(BeNumerically("==", 0.123457))
			Expect(segment["change"]).To(BeNumerically("==", 0.576543))
		})
	})

	When("a metric falls by less than a millionth", func() {
		It("prints the change as 0 in JSON, not -0", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
				ledgerLine("pre_run", "2026-11-02", "cell-a", trendRecord("run-2", 0.4999999)),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger, "--format", "json")

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			expectMatchesSchema(trendOutputSchemaPath, []byte(outcome.stdout))
			Expect(outcome.stdout).To(ContainSubstring(`"change":0}`))
		})
	})

	When("a metric value is a tiny negative number", func() {
		It("prints the value and the change as 0 in JSON, not -0", func() {
			tinyNegative := func(runID string) record {
				return withPath(completeRecord(runID), "parameters.k", -0.0000001)
			}
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", tinyNegative("run-1")),
				ledgerLine("pre_run", "2026-11-02", "cell-a", tinyNegative("run-2")),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger, "--metric", "parameters.k", "--format", "json")

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			expectMatchesSchema(trendOutputSchemaPath, []byte(outcome.stdout))
			Expect(outcome.stdout).NotTo(ContainSubstring(":-0"))
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
