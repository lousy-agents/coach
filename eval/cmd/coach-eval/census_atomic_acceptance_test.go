package main_test

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("a census run lands entirely or not at all", func() {
	var (
		dir, corpus string
		outputs     censusOutputs
		records     string
	)

	BeforeEach(func() {
		dir = GinkgoT().TempDir()
		path, sha := censusRepo(goRepoFiles(complexFunctions(1)))
		corpus = writeCorpus(corpusRepo{ID: "busy", Path: path, SHA: sha, Language: "go"})
		var outcome compareOutcome
		outcome, _, outputs = runCensusInto(dir, corpus, coachBinary())
		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		records = filepath.Join(outputs.censusRoot, "fixture", "records.jsonl")
	})

	When("the run ledger cannot be written", func() {
		It("leaves the records file byte-identical and exits non-zero", func() {
			before := readBytes(records)
			blocker := filepath.Join(dir, "not-a-directory")
			Expect(os.WriteFile(blocker, []byte("x"), 0o644)).To(Succeed())

			outcome := runProgram("", os.Environ(), coachEvalBinary, "census", "--coach", coachBinary(),
				"--census-root", outputs.censusRoot, "--ledger", filepath.Join(blocker, "runs.jsonl"), corpus)

			Expect(outcome.exitCode).NotTo(Equal(0))
			Expect(readBytes(records)).To(Equal(before), "a re-run must never duplicate records")
		})
	})

	When("the first run over a corpus cannot open the run ledger", func() {
		It("leaves no records file behind", func() {
			fresh := GinkgoT().TempDir()
			ledger := filepath.Join(fresh, "ledger", "runs.jsonl")
			Expect(os.MkdirAll(ledger, 0o755)).To(Succeed(), "a directory where the ledger file should be")
			censusRoot := filepath.Join(fresh, "census")

			outcome := runProgram("", os.Environ(), coachEvalBinary, "census", "--coach", coachBinary(), "--census-root", censusRoot, "--ledger", ledger, corpus)

			Expect(outcome.exitCode).NotTo(Equal(0))
			Expect(outcome.stderr).To(ContainSubstring("ledger"))
			Expect(filepath.Join(censusRoot, "fixture", "records.jsonl")).NotTo(BeAnExistingFile())
		})
	})

	When("the ledger line's write stops part way through", func() {
		It("leaves both the records file and the run ledger byte-identical", func() {
			const limit = 2048 * 512
			ledger := readBytes(outputs.ledger)
			padded := append(ledger, []byte(strings.Repeat("x", limit-len(ledger)-64)+"\n")...)
			Expect(os.WriteFile(outputs.ledger, padded, 0o644)).To(Succeed())
			before := readBytes(records)

			outcome := runProgram("", os.Environ(), "/bin/sh", "-c", `ulimit -f 2048 && exec "$@"`, "sh",
				coachEvalBinary, "census", "--coach", coachBinary(), "--census-root", outputs.censusRoot, "--ledger", outputs.ledger, corpus)

			Expect(outcome.exitCode).NotTo(Equal(0), outcome.stdout)
			Expect(readBytes(outputs.ledger)).To(Equal(padded))
			Expect(readBytes(records)).To(Equal(before))
		})
	})

	When("the run is recorded in the ledger", func() {
		It("names the coach binary by base name, with its sha256 as identity", func() {
			coach := jsonLines(outputs.ledger)[0]["run"].(map[string]any)["coach"].(map[string]any)
			Expect(coach["path"]).To(Equal(filepath.Base(coachBinary())))
			Expect(coach["sha256"]).To(HaveLen(64))
		})
	})
})

func readBytes(path string) []byte {
	raw, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())
	return raw
}
