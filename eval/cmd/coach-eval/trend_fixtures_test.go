package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/gomega"
)

// trendRecord is a comparable, complete run that carries a clean_success metric.
func trendRecord(runID string, cleanSuccess float64) record {
	return withPath(completeRecord(runID), "results.clean_success", cleanSuccess)
}

// trialsRecord is a comparable, complete run that carries a trials count and no clean_success.
func trialsRecord(runID string, trials int) record {
	return withPath(completeRecord(runID), "results.trials", trials)
}

// ledgerLine wraps a full run record as a self-contained ledger line.
func ledgerLine(kind, recorded, cell string, run record) record {
	return record{"schema_version": "1", "kind": kind, "recorded": recorded, "cell": cell, "run": run}
}

func writeLedger(path string, lines ...record) {
	var content strings.Builder
	for _, line := range lines {
		data, err := json.Marshal(line)
		Expect(err).NotTo(HaveOccurred())
		content.Write(data)
		content.WriteByte('\n')
	}
	Expect(os.MkdirAll(filepath.Dir(path), 0o755)).To(Succeed())
	Expect(os.WriteFile(path, []byte(content.String()), 0o600)).To(Succeed())
}

// runTrend runs the built coach-eval binary's trend command, from dir.
func runTrend(stateRoot, dir string, args ...string) compareOutcome {
	return runCommand(stateRoot, dir, coachEvalBinary, append([]string{"trend"}, args...)...)
}

// runTrendThroughMise runs the eval-trend mise task from the repository root.
func runTrendThroughMise(stateRoot string, args ...string) compareOutcome {
	return runCommand(stateRoot, filepath.Join("..", "..", ".."), "mise", append([]string{"run", "eval-trend", "--"}, args...)...)
}
