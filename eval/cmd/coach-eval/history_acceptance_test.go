package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"

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
			Expect(rule["unplaced_reasons"]).To(BeEmpty(), "every finding was placed, so no reason is recorded")
			Expect(rule["risk_ratio"]).To(BeNumerically("~", 1.5, 1e-9))
			Expect(rule["unplaced_findings"]).To(BeEquivalentTo(0), "the copy under eval/ is outside the census scope, so its findings are dropped rather than left unplaced")
		})
	})

	When("history_until is the census SHA itself", func() {
		It("exits 0 and records the empty window with no match rate and the reason", func() {
			outcome, outputs := runHistory(nil, writeHistoryCorpus(historyRepoEntry{ID: "r", Path: path, SHA: since, HistoryUntil: since, Language: "go"}), "--seed", "7")

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			repo := jsonLines(outputs.ledger)[0]["run"].(map[string]any)["repositories"].([]any)[0].(map[string]any)
			Expect(repo["commits"]).To(BeEquivalentTo(0))
			Expect(repo).To(HaveKeyWithValue("match_rate", BeNil()))
			Expect(repo["match_rate_null_reason"]).To(Equal("no commits between since and until"))
		})
	})

	When("the corpus names a Go rule the placement table does not hold", func() {
		It("refuses the repository before any rule is compared, names the rule, and writes no rule for it", func() {
			corpus := writeHistoryCorpusRules([]string{"complexity.cognitive_complexity", "architecture.made_up"}, historyRepoEntry{ID: "r", Path: path, SHA: since, HistoryUntil: until, Language: "go"})

			outcome, outputs := runHistory(nil, corpus, "--seed", "7")

			Expect(outcome.exitCode).To(Equal(3), outcome.stderr)
			repo := jsonLines(outputs.ledger)[0]["run"].(map[string]any)["repositories"].([]any)[0].(map[string]any)
			Expect(repo["evidence"]).To(ContainSubstring("architecture.made_up"))
			Expect(repo["rules"]).To(BeEmpty(), "the rule before it is not written either")
		})
	})

	When("the corpus names Go rules in an order that is not alphabetical", func() {
		It("writes the rules in the corpus's order, so the ledger line follows it", func() {
			corpus := writeHistoryCorpusRules([]string{"complexity.cognitive_complexity", "complexity.branch_density"}, historyRepoEntry{ID: "r", Path: path, SHA: since, HistoryUntil: until, Language: "go"})

			outcome, outputs := runHistory(nil, corpus, "--seed", "7")

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			rules := jsonLines(outputs.ledger)[0]["run"].(map[string]any)["repositories"].([]any)[0].(map[string]any)["rules"].([]any)
			Expect([]any{rules[0].(map[string]any)["rule_id"], rules[1].(map[string]any)["rule_id"]}).To(Equal([]any{"complexity.cognitive_complexity", "complexity.branch_density"}))
		})
	})

	When("the same history runs twice over one corpus, census root and ledger", func() {
		It("links each record of the second run to the first run's record for the same target", func() {
			corpus := writeHistoryCorpus(historyRepoEntry{ID: "r", Path: path, SHA: since, HistoryUntil: until, Language: "go"})
			first, outputs := runHistory(nil, corpus, "--seed", "7")
			Expect(first.exitCode).To(Equal(0), first.stderr)
			var firstIDs []any
			for _, record := range jsonLines(outputs.records()) {
				firstIDs = append(firstIDs, record["record_id"])
			}

			second := runProgram("", os.Environ(), coachEvalBinary, "history", "--coach", coachBinary(), "--census-root", outputs.censusRoot, "--ledger", outputs.ledger, "--seed", "7", corpus)

			Expect(second.exitCode).To(Equal(0), second.stderr)
			all := jsonLines(outputs.records())
			Expect(all).To(HaveLen(2 * len(firstIDs)))
			for _, record := range all[len(firstIDs):] {
				Expect(record["supersedes"]).To(BeElementOf(firstIDs...), "a re-run's record names the first run's record for its target")
			}
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
})
