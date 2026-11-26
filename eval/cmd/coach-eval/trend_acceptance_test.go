package main_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("eval-compare trend mode", func() {
	var root, ledger string

	BeforeEach(func() {
		var err error
		root, err = os.MkdirTemp("", "coach-eval-trend")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(os.RemoveAll, root)
		ledger = filepath.Join(root, "runs.jsonl")
	})

	When("a cell's adjacent runs are comparable", func() {
		It("prints one segment with the metric change and exits 0", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
				ledgerLine("pre_run", "2026-11-02", "cell-a", trendRecord("run-2", 0.75)),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(outcome.stdout).To(ContainSubstring("segments 1"))
			Expect(outcome.stdout).To(ContainSubstring("0.5"))
			Expect(outcome.stdout).To(ContainSubstring("0.75"))
		})
	})

	When("a run differs from its predecessor in a comparable field", func() {
		It("starts a new segment and names the field that broke it", func() {
			changed := withPath(trendRecord("run-3", 0.9), "solver.model", "claude-other-model")
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
				ledgerLine("pre_run", "2026-11-02", "cell-a", trendRecord("run-2", 0.75)),
				ledgerLine("pre_run", "2026-11-03", "cell-a", changed),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(outcome.stdout).To(ContainSubstring("segments 2"))
			Expect(outcome.stdout).To(ContainSubstring("break before run-3: solver.model differs"))
		})
	})

	When("a run is not complete", func() {
		It("breaks the trend at that run and names its state", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
				ledgerLine("pre_run", "2026-11-02", "cell-a", trendRecord("run-2", 0.75)),
				ledgerLine("pre_run", "2026-11-03", "cell-a", withPath(trendRecord("run-3", 0.9), "state", "paused")),
				ledgerLine("pre_run", "2026-11-04", "cell-a", trendRecord("run-4", 0.8)),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(outcome.stdout).To(ContainSubstring("break before run-3: state paused"))
			Expect(outcome.stdout).To(ContainSubstring("segments 2"))
		})
	})

	When("a run lacks the metric", func() {
		It("starts a new segment with the reason metric absent", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
				ledgerLine("pre_run", "2026-11-02", "cell-a", trendRecord("run-2", 0.75)),
				ledgerLine("pre_run", "2026-11-03", "cell-a", completeRecord("run-3")),
				ledgerLine("pre_run", "2026-11-04", "cell-a", trendRecord("run-4", 0.8)),
				ledgerLine("pre_run", "2026-11-05", "cell-a", trendRecord("run-5", 0.9)),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(outcome.stdout).To(ContainSubstring("break before run-3: metric absent"))
			Expect(outcome.stdout).To(ContainSubstring("segments 2"))
		})
	})

	When("a metric is given with --metric", func() {
		It("trends that dot path instead of results.clean_success", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trialsRecord("run-1", 4)),
				ledgerLine("pre_run", "2026-11-02", "cell-a", trialsRecord("run-2", 6)),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger, "--metric", "results.trials")

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(outcome.stdout).To(ContainSubstring("results.trials"))
			Expect(outcome.stdout).To(ContainSubstring("6"))
		})
	})

	When("a run's worktree is dirty", func() {
		It("warns about that run and keeps it in the segment", func() {
			dirty := withPath(trendRecord("run-2", 0.75), "worktree_dirty", true)
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
				ledgerLine("pre_run", "2026-11-02", "cell-a", dirty),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(outcome.stderr).To(ContainSubstring("run-2"))
			Expect(outcome.stdout).To(ContainSubstring("segments 1"))
		})
	})

	When("the ledger holds census lines for the same cell", func() {
		It("trends only the pre_run, study and baseline lines", func() {
			census := withPath(trendRecord("run-x", 0.1), "solver.model", "claude-other-model")
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
				ledgerLine("census", "2026-11-02", "cell-a", census),
				ledgerLine("pre_run", "2026-11-03", "cell-a", trendRecord("run-2", 0.75)),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(outcome.stdout).To(ContainSubstring("segments 1"))
		})
	})

	When("the cell has one run and no adjacent pair", func() {
		It("exits 4 and says there is no comparable adjacent pair", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(4), outcome.stderr)
			Expect(outcome.stderr).To(ContainSubstring("no comparable adjacent pair"))
		})
	})

	When("the cell is not in the ledger", func() {
		It("exits 2 and names the cell", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
			)

			outcome := runTrend(root, root, "--cell", "cell-zzz", "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
			Expect(outcome.stderr).To(ContainSubstring("cell-zzz"))
		})
	})

	When("the ledger file is missing", func() {
		It("exits 2 and names the ledger path", func() {
			missing := filepath.Join(root, "missing.jsonl")

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", missing)

			Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
			Expect(outcome.stderr).To(ContainSubstring("missing.jsonl"))
		})
	})

	When("no ledger is given and the default path is absent", func() {
		It("exits 2 and names eval/ledger/runs.jsonl", func() {
			outcome := runTrend(root, root, "--cell", "cell-a")

			Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
			Expect(outcome.stderr).To(ContainSubstring("eval/ledger/runs.jsonl"))
		})
	})

	When("a ledger line has an unknown kind", func() {
		It("exits 2 and names the kind key", func() {
			writeLedger(ledger,
				ledgerLine("bogus", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
			Expect(outcome.stderr).To(ContainSubstring("kind"))
		})
	})

	When("a ledger line's run fails the run schema", func() {
		It("exits 2 and names the offending key", func() {
			invalid := withPath(trendRecord("run-1", 0.5), "results.clean_success", 2)
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", invalid),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
			Expect(outcome.stderr).To(ContainSubstring("clean_success"))
		})
	})

	When("lines are out of date order in the file", func() {
		It("orders the segments by recorded date", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-02", "cell-a", trendRecord("run-2", 0.75)),
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(outcome.stdout).To(ContainSubstring("segment 1: run-1"))
		})
	})

	When("the cell argument is missing", func() {
		It("exits 2 and names the cell flag", func() {
			outcome := runTrend(root, root, "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
			Expect(outcome.stderr).To(ContainSubstring("cell"))
		})
	})

	When("a metric change has float noise", func() {
		It("prints the change rounded to six decimal places", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
				ledgerLine("pre_run", "2026-11-02", "cell-a", trendRecord("run-2", 0.7)),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(outcome.stdout).To(ContainSubstring("(change +0.2)"))
		})
	})

	When("a metric falls by less than a millionth", func() {
		It("prints the change as +0, not -0", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
				ledgerLine("pre_run", "2026-11-02", "cell-a", trendRecord("run-2", 0.4999999)),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(outcome.stdout).To(ContainSubstring("(change +0)"))
		})
	})

	When("a metric value is a tiny negative number", func() {
		It("prints the value and the change as 0, not -0", func() {
			tinyNegative := func(runID string) record {
				return withPath(completeRecord(runID), "parameters.k", -0.0000001)
			}
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", tinyNegative("run-1")),
				ledgerLine("pre_run", "2026-11-02", "cell-a", tinyNegative("run-2")),
			)

			outcome := runTrend(root, root, "--cell", "cell-a", "--ledger", ledger, "--metric", "parameters.k")

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(outcome.stdout).To(ContainSubstring("0 -> 0 (change +0)"))
		})
	})

	When("run through the eval-trend mise task", func() {
		It("reaches the trend command with its arguments", func() {
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
				ledgerLine("pre_run", "2026-11-02", "cell-a", trendRecord("run-2", 0.75)),
			)

			outcome := runTrendThroughMise(root, "--cell", "cell-a", "--ledger", ledger)

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(outcome.stdout).To(ContainSubstring("segments 1"))
		})
	})
})
