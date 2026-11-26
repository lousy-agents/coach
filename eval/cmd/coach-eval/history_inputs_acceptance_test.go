package main_test

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach-eval history inputs and isolation", func() {
	var path, since, until string

	BeforeEach(func() {
		path, since, _, until = historyRepo()
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
		Entry("a repository path that is a file, not a directory", func() historyRepoEntry {
			return historyRepoEntry{ID: "r", Path: filepath.Join(path, "go.mod"), SHA: since, HistoryUntil: until, Language: "go"}
		}, []string{"--seed", "7"}, "not a local directory"),
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

	DescribeTable("a command line without a required part exits 2 and writes nothing",
		func(drop string, message string) {
			corpus := writeHistoryCorpus(historyRepoEntry{ID: "r", Path: path, SHA: since, HistoryUntil: until, Language: "go"})
			dir := GinkgoT().TempDir()
			ledger := filepath.Join(dir, "ledger", "runs.jsonl")
			args := historyArgsWithout(drop, map[string]string{"--coach": coachBinary(), "--census-root": filepath.Join(dir, "census"), "--ledger": ledger}, corpus)

			outcome := runProgram("", os.Environ(), coachEvalBinary, args...)

			Expect(outcome.exitCode).To(Equal(2), outcome.stdout)
			Expect(outcome.stderr).To(ContainSubstring(message))
			Expect(ledger).NotTo(BeAnExistingFile())
		},
		Entry("no --coach", "--coach", "--coach <coach binary> is required"),
		Entry("no --census-root", "--census-root", "--census-root <directory> is required"),
		Entry("no --ledger", "--ledger", "--ledger <runs.jsonl> is required"),
		Entry("two corpus manifests", "one manifest", "exactly one corpus manifest"),
	)

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
			expectOnlyLocalGit(calls, path)
			Expect(strings.Join(calls, "\n")).To(ContainSubstring("protocol.allow=never"), "the history's own git calls refuse every transport")
		})
	})
})
