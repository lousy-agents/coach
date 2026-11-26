package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func jsonLines(path string) []map[string]any {
	raw, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())
	var lines []map[string]any
	for _, text := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var line map[string]any
		Expect(json.Unmarshal([]byte(text), &line)).To(Succeed(), text)
		lines = append(lines, line)
	}
	return lines
}

var _ = Describe("what a census run leaves behind", func() {
	When("a census of two repositories completes", func() {
		It("appends one analysis_status record per repository and rule, and one census ledger line", func() {
			busyPath, busySHA := censusRepo(goRepoFiles(complexFunctions(2)))
			quietPath, quietSHA := censusRepo(goRepoFiles(complexFunctions(0)))
			corpus := writeCorpus(corpusRepo{ID: "busy", Path: busyPath, SHA: busySHA, Language: "go"}, corpusRepo{ID: "quiet", Path: quietPath, SHA: quietSHA, Language: "go"})

			outcome, summary, outputs := runCensusInto(GinkgoT().TempDir(), corpus, coachBinary())
			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)

			records := jsonLines(filepath.Join(outputs.censusRoot, "fixture", "records.jsonl"))
			Expect(records).To(HaveLen(2))
			values := map[string]any{}
			for _, record := range records {
				target := record["target"].(map[string]any)
				Expect(record["measure"]).To(Equal("analysis_status"))
				Expect(record["source"]).To(Equal(map[string]any{"kind": "census", "ref": summary.Coach.SHA256}))
				Expect(target["path"]).To(Equal("."))
				Expect(target["lines"]).To(BeNil())
				Expect(target["rule_id"]).To(Equal("complexity.cognitive_complexity"))
				values[target["sha"].(string)] = record["value"]
			}
			Expect(values).To(Equal(map[string]any{busySHA: "measured", quietSHA: "not_exercised"}))

			ledger := jsonLines(outputs.ledger)
			Expect(ledger).To(HaveLen(1))
			Expect(ledger[0]["kind"]).To(Equal("census"))
			Expect(ledger[0]["cell"]).To(Equal("census:fixture"))
			run := ledger[0]["run"].(map[string]any)
			Expect(run["corpus_id"]).To(Equal("fixture"))
			Expect(run["repositories"]).To(HaveLen(2))
			Expect(run["exit"]).To(BeEquivalentTo(0))
			Expect(ledger[0]["recorded"]).To(Equal(time.Now().UTC().Format(time.DateOnly)))
		})
	})

	When("one repository's analysis fails", func() {
		It("records the failure evidence as the rationale, and a measured rule's counts as its own", func() {
			brokenPath, brokenSHA := censusRepo(goRepoFiles(complexFunctions(1)))
			busyPath, busySHA := censusRepo(goRepoFiles(complexFunctions(2)))
			corpus := writeCorpus(
				corpusRepo{ID: "broken", Path: brokenPath, SHA: brokenSHA, Language: "go", ProjectConfig: ".lousy-agents/missing.json"},
				corpusRepo{ID: "busy", Path: busyPath, SHA: busySHA, Language: "go"},
			)

			outcome, _, outputs := runCensusInto(GinkgoT().TempDir(), corpus, coachBinary())
			Expect(outcome.exitCode).To(Equal(3), outcome.stderr)

			rationales := map[string]any{}
			for _, record := range jsonLines(filepath.Join(outputs.censusRoot, "fixture", "records.jsonl")) {
				rationales[record["target"].(map[string]any)["sha"].(string)] = record["rationale"]
			}
			Expect(rationales[brokenSHA]).To(ContainSubstring("missing.json"))
			Expect(rationales[busySHA]).To(Equal("findings=2 adjudicable=2 sampled=2"))

			ledger := jsonLines(outputs.ledger)
			Expect(ledger).To(HaveLen(1))
			Expect(ledger[0]["run"].(map[string]any)["exit"]).To(BeEquivalentTo(3), "a failed census is never recorded as a clean one")
		})
	})

	When("a second census runs over the same corpus", func() {
		It("appends to the records and the ledger, keeping the first run's lines", func() {
			path, sha := censusRepo(goRepoFiles(complexFunctions(1)))
			corpus := writeCorpus(corpusRepo{ID: "busy", Path: path, SHA: sha, Language: "go"})
			dir := GinkgoT().TempDir()

			runCensusInto(dir, corpus, coachBinary())
			_, _, outputs := runCensusInto(dir, corpus, coachBinary())

			Expect(jsonLines(filepath.Join(outputs.censusRoot, "fixture", "records.jsonl"))).To(HaveLen(2))
			ledger := jsonLines(outputs.ledger)
			Expect(ledger).To(HaveLen(2))
			Expect(ledger[0]["run"].(map[string]any)["run_id"]).NotTo(Equal(ledger[1]["run"].(map[string]any)["run_id"]))
		})
	})

	When("a write to the run ledger stops part way through", func() {
		It("truncates the run ledger back to its length before the append, and fails", func() {
			dir := GinkgoT().TempDir()
			ledger := filepath.Join(dir, "ledger", "runs.jsonl")
			Expect(os.MkdirAll(filepath.Dir(ledger), 0o755)).To(Succeed())
			const limit = 2048 * 512
			before := strings.Repeat("x", limit-64) + "\n"
			Expect(os.WriteFile(ledger, []byte(before), 0o644)).To(Succeed())
			path, sha := censusRepo(goRepoFiles(complexFunctions(1)))
			corpus := writeCorpus(corpusRepo{ID: "busy", Path: path, SHA: sha, Language: "go"})

			outcome := runProgram("", os.Environ(), "/bin/sh", "-c", `ulimit -f 2048 && exec "$@"`, "sh",
				coachEvalBinary, "census", "--coach", coachBinary(), "--census-root", filepath.Join(dir, "census"), "--ledger", ledger, corpus)

			Expect(outcome.exitCode).NotTo(Equal(0), outcome.stdout)
			Expect(outcome.stderr).To(ContainSubstring("ledger"))
			raw, err := os.ReadFile(ledger)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(raw)).To(Equal(len(before)), "no torn line is left for eval-trend to refuse")
			Expect(ledger + ".lock").NotTo(BeAnExistingFile())
		})
	})

	When("another writer holds the run ledger's lock", func() {
		It("waits, then fails naming the lock, and leaves the run ledger untouched", func() {
			dir := GinkgoT().TempDir()
			ledger := filepath.Join(dir, "ledger", "runs.jsonl")
			writeLedger(ledger, ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)))
			Expect(os.WriteFile(ledger+".lock", nil, 0o644)).To(Succeed())
			before, err := os.ReadFile(ledger)
			Expect(err).NotTo(HaveOccurred())
			path, sha := censusRepo(goRepoFiles(complexFunctions(1)))
			corpus := writeCorpus(corpusRepo{ID: "busy", Path: path, SHA: sha, Language: "go"})

			outcome, _, _ := runCensusInto(dir, corpus, coachBinary())

			Expect(outcome.exitCode).NotTo(Equal(0))
			Expect(outcome.stderr).To(ContainSubstring(ledger + ".lock"))
			Expect(os.ReadFile(ledger)).To(Equal(before))
		})
	})

	When("the ledger already holds study lines", func() {
		It("appends a census line that the trend still reads past", func() {
			dir := GinkgoT().TempDir()
			ledger := filepath.Join(dir, "ledger", "runs.jsonl")
			writeLedger(ledger,
				ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)),
				ledgerLine("pre_run", "2026-11-02", "cell-a", trendRecord("run-2", 0.75)),
			)
			path, sha := censusRepo(goRepoFiles(complexFunctions(1)))
			corpus := writeCorpus(corpusRepo{ID: "busy", Path: path, SHA: sha, Language: "go"})

			outcome, _, _ := runCensusInto(dir, corpus, coachBinary())
			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)

			trend := runTrend(dir, dir, "--cell", "cell-a", "--ledger", ledger)
			Expect(trend.exitCode).To(Equal(0), trend.stderr)
			Expect(trend.stdout).To(ContainSubstring("segments 1"))
		})
	})

	When("the ledger's last line has no newline", func() {
		It("appends the census line as a line of its own", func() {
			dir := GinkgoT().TempDir()
			ledger := filepath.Join(dir, "ledger", "runs.jsonl")
			writeLedger(ledger, ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)))
			raw, err := os.ReadFile(ledger)
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(ledger, []byte(strings.TrimSuffix(string(raw), "\n")), 0o644)).To(Succeed())
			path, sha := censusRepo(goRepoFiles(complexFunctions(1)))
			corpus := writeCorpus(corpusRepo{ID: "busy", Path: path, SHA: sha, Language: "go"})

			outcome, _, _ := runCensusInto(dir, corpus, coachBinary())
			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)

			Expect(jsonLines(ledger)).To(HaveLen(2))
		})
	})

	DescribeTable("a ledger line whose run does not fit its kind is refused",
		func(line record) {
			dir := GinkgoT().TempDir()
			ledger := filepath.Join(dir, "runs.jsonl")
			writeLedger(ledger, ledgerLine("pre_run", "2026-11-01", "cell-a", trendRecord("run-1", 0.5)), line)

			trend := runTrend(dir, dir, "--cell", "cell-a", "--ledger", ledger)
			Expect(trend.exitCode).To(Equal(2))
			Expect(trend.stderr).To(ContainSubstring("line 2"))
		},
		Entry("a census line whose run has no corpus_id", ledgerLine("census", "2026-11-02", "census:fixture",
			record{"schema_version": "1", "run_id": "census-1", "coach": record{"path": "/c", "sha256": "ab"}, "repositories": []any{}, "exit": 0})),
		Entry("a study line carrying a census run", ledgerLine("pre_run", "2026-11-02", "cell-a",
			record{"schema_version": "1", "run_id": "census-1", "corpus_id": "fixture", "coach": record{"path": "/c", "sha256": "ab"}, "repositories": []any{}, "exit": 0})),
	)

	When("a coach that cannot run meets a repository whose censused rules are all out of scope", func() {
		It("still exits 3, so a coach that never ran is never a clean census", func() {
			path, sha := censusRepo(goRepoFiles(complexFunctions(1)))
			corpus := writeCorpusFor("coupling.deep_relative_import", "typescript", corpusRepo{ID: "busy", Path: path, SHA: sha, Language: "go"})
			notExecutable := filepath.Join(GinkgoT().TempDir(), "coach")
			Expect(os.WriteFile(notExecutable, []byte("not a program"), 0o644)).To(Succeed())

			outcome, _ := runCensus(corpus, notExecutable)

			Expect(outcome.exitCode).To(Equal(3), outcome.stderr)
		})
	})
})
