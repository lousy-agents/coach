package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach-eval history", func() {
	var path, since, fix, until string

	BeforeEach(func() {
		path, since, fix, until = historyRepo()
	})

	When("every input is usable", func() {
		var (
			outcome compareOutcome
			outputs historyOutputs
		)

		BeforeEach(func() {
			outcome, outputs = runHistory(nil, writeHistoryCorpus(historyRepoEntry{ID: "r", Path: path, SHA: since, HistoryUntil: until, Language: "go"}), "--seed", "7")
		})

		It("exits 0 and prints the summary, stating association, not causation", func() {
			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			var summary map[string]any
			Expect(json.Unmarshal([]byte(outcome.stdout), &summary)).To(Succeed(), outcome.stdout)
			Expect(summary["statement"]).To(ContainSubstring("association, not causation"))
			Expect(summary["seed"]).To(BeEquivalentTo(7))
		})

		It("appends one association record per flagged finding, valued by the fix that touched it", func() {
			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			values := map[string]any{}
			for _, record := range jsonLines(outputs.records()) {
				Expect(record["measure"]).To(Equal("flagged_vs_unflagged_association"))
				target := record["target"].(map[string]any)
				Expect(target["sha"]).To(Equal(since))
				values[target["subject"].(string)] = []any{record["value"], record["source"].(map[string]any)["ref"]}
			}
			Expect(values).To(Equal(map[string]any{"F0": []any{1.0, fix}, "F1": []any{0.0, "none"}}))
		})

		It("appends one history ledger line with the regex, its match rate, the unmapped fixes and the draw", func() {
			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			ledger := jsonLines(outputs.ledger)
			Expect(ledger).To(HaveLen(1))
			Expect(ledger[0]["kind"]).To(Equal("history"))
			Expect(ledger[0]["cell"]).To(Equal("history:fixture"))
			run := ledger[0]["run"].(map[string]any)
			Expect(run["exit"]).To(BeEquivalentTo(0))
			Expect(run["seed"]).To(BeEquivalentTo(7))
			Expect(run["controls_per_flagged"]).To(BeEquivalentTo(3))
			Expect(run["fix_pattern"]).NotTo(BeEmpty())
			Expect(run["statement"]).To(ContainSubstring("association, not causation"))
			Expect(run["coach"].(map[string]any)["path"]).To(Equal(filepath.Base(coachBinary())))
			repo := run["repositories"].([]any)[0].(map[string]any)
			Expect(repo["commits"]).To(BeEquivalentTo(2))
			Expect(repo["fixes"]).To(BeEquivalentTo(1))
			Expect(repo["match_rate"]).To(BeNumerically("~", 0.5, 1e-9))
			Expect(repo["unmapped_fixes"]).To(Equal(map[string]any{"function": 0.0, "file": 0.0}))
			rule := repo["rules"].([]any)[0].(map[string]any)
			Expect([]any{rule["flagged"], rule["flagged_involved"], rule["controls"], rule["controls_involved"]}).To(Equal([]any{2.0, 1.0, 3.0, 1.0}))
			Expect(rule["risk_ratio"]).To(BeNumerically("~", 1.5, 1e-9))
		})
	})

	When("Coach fails on the repository", func() {
		It("exits 3 and records the run as failed", func() {
			outcome, outputs := runHistory(nil, writeHistoryCorpus(historyRepoEntry{ID: "r", Path: path, SHA: since, HistoryUntil: until, Language: "go", ProjectConfig: ".lousy-agents/missing.json"}), "--seed", "7")

			Expect(outcome.exitCode).To(Equal(3), outcome.stderr)
			run := jsonLines(outputs.ledger)[0]["run"].(map[string]any)
			Expect(run["exit"]).To(BeEquivalentTo(3))
			Expect(run["repositories"].([]any)[0].(map[string]any)["evidence"]).To(ContainSubstring("missing.json"))
		})
	})

	DescribeTable("an unusable input exits 2 and writes nothing",
		func(entry func() historyRepoEntry, extra []string, message string) {
			outcome, outputs := runHistory(nil, writeHistoryCorpus(entry()), extra...)

			Expect(outcome.exitCode).To(Equal(2), outcome.stdout)
			Expect(outcome.stderr).To(ContainSubstring(message))
			Expect(outputs.ledger).NotTo(BeAnExistingFile())
			Expect(outputs.records()).NotTo(BeAnExistingFile())
		},
		Entry("no --seed", func() historyRepoEntry {
			return historyRepoEntry{ID: "r", Path: path, SHA: since, HistoryUntil: until, Language: "go"}
		}, nil, "--seed"),
		Entry("a --seed that is not an integer", func() historyRepoEntry {
			return historyRepoEntry{ID: "r", Path: path, SHA: since, HistoryUntil: until, Language: "go"}
		}, []string{"--seed", "x"}, "--seed"),
		Entry("--online", func() historyRepoEntry {
			return historyRepoEntry{ID: "r", Path: path, SHA: since, HistoryUntil: until, Language: "go"}
		}, []string{"--seed", "7", "--online"}, "offline"),
		Entry("a repository with no history_until", func() historyRepoEntry {
			return historyRepoEntry{ID: "r", Path: path, SHA: since, Language: "go"}
		}, []string{"--seed", "7"}, "history_until"),
		Entry("a TypeScript repository", func() historyRepoEntry {
			return historyRepoEntry{ID: "r", Path: path, SHA: since, HistoryUntil: until, Language: "typescript"}
		}, []string{"--seed", "7"}, "TypeScript"),
		Entry("an unreachable history_until", func() historyRepoEntry {
			return historyRepoEntry{ID: "r", Path: path, SHA: since, HistoryUntil: strings.Repeat("a", 40), Language: "go"}
		}, []string{"--seed", "7"}, "is not a commit"),
		Entry("a repository path that is a URL, not a local clone", func() historyRepoEntry {
			return historyRepoEntry{ID: "r", Path: "https://example.invalid/r.git", SHA: since, HistoryUntil: until, Language: "go"}
		}, []string{"--seed", "7"}, "local"),
	)

	When("a census and a history run at once over one corpus, census root and ledger", func() {
		It("lands both runs whole: two ledger lines, and every record of each", func() {
			corpus := writeHistoryCorpus(historyRepoEntry{ID: "r", Path: path, SHA: since, HistoryUntil: until, Language: "go"})
			dir := GinkgoT().TempDir()
			censusRoot, ledger := filepath.Join(dir, "census"), filepath.Join(dir, "ledger", "runs.jsonl")
			outcomes := make(chan compareOutcome, 2)
			for _, task := range [][]string{{"census"}, {"history", "--seed", "7"}} {
				go func(task []string) {
					defer GinkgoRecover()
					args := append(task, "--coach", coachBinary(), "--census-root", censusRoot, "--ledger", ledger, corpus)
					outcomes <- runProgram("", os.Environ(), coachEvalBinary, args...)
				}(task)
			}
			for range 2 {
				outcome := <-outcomes
				Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			}

			kinds := []any{}
			for _, line := range jsonLines(ledger) {
				kinds = append(kinds, line["kind"])
			}
			Expect(kinds).To(ConsistOf("census", "history"))
			measures := map[any]int{}
			for _, record := range jsonLines(filepath.Join(censusRoot, "fixture", "records.jsonl")) {
				measures[record["measure"]]++
			}
			Expect(measures).To(Equal(map[any]int{"analysis_status": 1, "flagged_vs_unflagged_association": 2}))
		})
	})

	When("git is watched on PATH", func() {
		It("never fetches, lists a remote, or clones anything but a local directory", func() {
			isolatedGit(path, "remote", "add", "origin", "https://example.invalid/r.git")
			pathEnv, log := gitShim()

			outcome, _ := runHistory([]string{pathEnv}, writeHistoryCorpus(historyRepoEntry{ID: "r", Path: path, SHA: since, HistoryUntil: until, Language: "go"}), "--seed", "7")

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			raw, err := os.ReadFile(log)
			Expect(err).NotTo(HaveOccurred(), "the history ran git through PATH")
			calls := strings.Split(strings.TrimSpace(string(raw)), "\n")
			Expect(calls).NotTo(BeEmpty())
			for _, call := range calls {
				Expect(call).NotTo(MatchRegexp(`\b(fetch|pull|push|ls-remote|remote-https?)\b`), call)
				Expect(call).NotTo(ContainSubstring("://"), call)
				if strings.Contains(call, " clone ") || strings.HasPrefix(call, "clone ") {
					Expect(call).To(ContainSubstring(path), call)
				}
			}
			Expect(strings.Join(calls, "\n")).To(ContainSubstring("protocol.allow=never"), "the history's own git calls refuse every transport")
		})
	})
})
