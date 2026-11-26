package main_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("eval-compare trend guard rules", func() {
	var root, ledger string

	BeforeEach(func() {
		var err error
		root, err = os.MkdirTemp("", "coach-eval-trend-guard")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(os.RemoveAll, root)
		ledger = filepath.Join(root, "runs.jsonl")
	})

	When("only one of two adjacent runs carries the metric", func() {
		It("exits 4, because no adjacent pair has the metric on both runs", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", completeRecord("run-1")),
				ledgerLine("pre_run", "2026-11-02", "cell-a", trendRecord("run-2", 0.5)),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(4), outcome.stderr)
			Expect(outcome.stderr).To(ContainSubstring("no comparable adjacent pair"))
		})
	})

	When("a run lacks the metric next to a pair that has it", func() {
		It("exits 0 on the pair and breaks at the run without the metric", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
				ledgerLine("pre_run", "2026-11-02", "cell-a", trendRecord("run-2", 0.75)),
				ledgerLine("pre_run", "2026-11-03", "cell-a", completeRecord("run-3")),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(outcome.stdout).To(ContainSubstring("break before run-3: metric absent"))
		})
	})

	DescribeTable("a ledger kind that is trended", func(kind string) {
		writeLedger(ledger,
			ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
			ledgerLine(kind, "2026-11-02", "cell-a", trendRecord("run-2", 0.75)),
		)

		outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger)

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		Expect(outcome.stdout).To(ContainSubstring("runs 2, segments 1"))
	},
		Entry("study lines", "study"),
		Entry("baseline lines", "baseline"),
	)

	When("a history line is in the ledger for the same cell", func() {
		It("does not trend it, so the cell has one trend run and exits 4", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
				ledgerLine("history", "2026-11-02", "cell-a", trendRecord("run-2", 0.75)),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(4), outcome.stderr)
		})
	})

	When("two lines share a recorded date", func() {
		It("keeps their file order", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-2", 0.75)),
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(outcome.stdout).To(ContainSubstring("segment 1: run-2 to run-1"))
		})
	})

	When("a run after a non-complete run differs from the run before it", func() {
		It("starts a segment without a field break, and names only the state break", func() {
			changed := withPath(trendRecord("run-4", 0.8), "solver.model", "claude-other-model")
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
				ledgerLine("pre_run", "2026-11-02", "cell-a", trendRecord("run-2", 0.75)),
				ledgerLine("pre_run", "2026-11-03", "cell-a", withPath(trendRecord("run-3", 0.9), "state", "paused")),
				ledgerLine("pre_run", "2026-11-04", "cell-a", changed),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(outcome.stdout).To(ContainSubstring("break before run-3: state paused"))
			Expect(outcome.stdout).NotTo(ContainSubstring("break before run-4"))
			Expect(outcome.stdout).To(ContainSubstring("segment 2: run-4 (1 run)"))
		})
	})

	When("two comparable fields differ at once", func() {
		It("names both fields in one break reason, in path order", func() {
			changed := withPath(withPath(trendRecord("run-3", 0.9), "solver.model", "claude-other-model"), "caps.trials", 361)
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
				ledgerLine("pre_run", "2026-11-02", "cell-a", trendRecord("run-2", 0.75)),
				ledgerLine("pre_run", "2026-11-03", "cell-a", changed),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(outcome.stdout).To(ContainSubstring("break before run-3: caps.trials, solver.model differ"))
		})
	})

	When("the metric rises across a segment", func() {
		It("prints the first and last values with a signed change", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
				ledgerLine("pre_run", "2026-11-02", "cell-a", trendRecord("run-2", 0.75)),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(outcome.stdout).To(ContainSubstring("  0.5 -> 0.75 (change +0.25)"))
		})
	})

	When("the metric falls across a segment", func() {
		It("prints the change with a minus sign", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.75)),
				ledgerLine("pre_run", "2026-11-02", "cell-a", trendRecord("run-2", 0.5)),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(outcome.stdout).To(ContainSubstring("  0.75 -> 0.5 (change -0.25)"))
		})
	})

	When("an unknown flag is given", func() {
		It("exits 2 and names the flag", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger, "--bogus")

			Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
			Expect(outcome.stderr).To(ContainSubstring("bogus"))
		})
	})

	When("a ledger line after the first is invalid", func() {
		It("names the line number of the invalid line", func() {
			valid := ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5))
			invalid := ledgerLine("bogus", "2026-11-02", "cell-a", trendRecord("run-2", 0.75))
			writeLedger(ledger, valid, invalid)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
			Expect(outcome.stderr).To(ContainSubstring("line 2"))
		})
	})

	When("the eval-trend task is run without a cell", func() {
		It("exits 2 through the task", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
			)

			outcome := runTrendThroughMise(root, "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
			Expect(outcome.stderr).To(ContainSubstring("--cell is required"))
		})
	})

	When("the eval-trend task is run on a cell with no comparable adjacent pair", func() {
		It("exits 4 through the task", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
			)

			outcome := runTrendThroughMise(root, "--cell", "cell-a", "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(4), outcome.stderr)
			Expect(outcome.stderr).To(ContainSubstring("no comparable adjacent pair"))
		})
	})

	DescribeTable("a ledger line that breaks the ledger line schema", func(breakLine func(record), expectedText string) {
		line := ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5))
		breakLine(line)
		writeLedger(ledger, line)

		outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger)

		Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
		Expect(outcome.stderr).To(ContainSubstring(expectedText))
	},
		Entry("has an extra top-level key", func(r record) { r["extra"] = true }, "extra"),
		Entry("is missing the cell", func(r record) { delete(r, "cell") }, "missing property 'cell'"),
		Entry("is missing the recorded date", func(r record) { delete(r, "recorded") }, "recorded"),
		Entry("has a recorded date that is not YYYY-MM-DD", func(r record) { r["recorded"] = "2026/11/01" }, "recorded"),
		Entry("has an empty cell", func(r record) { r["cell"] = "" }, "minLength"),
		Entry("has schema_version 2", func(r record) { r["schema_version"] = "2" }, "schema_version"),
		Entry("has a negative trials count", func(r record) { r["run"] = withPath(r["run"].(record), "results.trials", -1) }, "trials"),
		Entry("has a clean_success below 0", func(r record) { r["run"] = withPath(r["run"].(record), "results.clean_success", -0.1) }, "clean_success"),
		Entry("has a trials count that is not an integer", func(r record) { r["run"] = withPath(r["run"].(record), "results.trials", 1.5) }, "trials"),
	)
})
