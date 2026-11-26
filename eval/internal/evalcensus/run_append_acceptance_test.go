package evalcensus_test

import (
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

var _ = Describe("a run's records and ledger line, written as one unit", func() {
	When("another records writer appends while a run's ledger line fails", func() {
		It("keeps the other writer's records, because the run holds the records lock from its snapshot to its rollback", func() {
			root := GinkgoT().TempDir()
			ledger := filepath.Join(root, "ledger", "runs.jsonl")
			Expect(os.MkdirAll(ledger, 0o755)).To(Succeed(), "a directory where the ledger file should be, so the line cannot be written")
			done := make(chan error, 1)
			restore := evalcensus.OnRecordsSnapshot(func() {
				go func() {
					done <- evalcensus.AppendCorpusRecords(root, "pilot", []evalcensus.Record{automatedRecord("other", `"agree"`)})
				}()
				time.Sleep(200 * time.Millisecond)
			})

			err := evalcensus.AppendRun(ledger, []byte("{}\n"), root, "pilot", []evalcensus.Record{automatedRecord("run", `"disagree"`)})
			restore()

			Expect(err).To(HaveOccurred())
			Expect(<-done).To(Succeed())
			records, err := evalcensus.LoadRecords(filepath.Join(root, "pilot", "records.jsonl"))
			Expect(err).NotTo(HaveOccurred())
			Expect(records).To(HaveLen(1))
			Expect(records[0].RecordID).To(Equal("other"))
		})
	})

	When("another run appends to the same ledger while this run's ledger line is being written and then fails", func() {
		It("keeps the other run's ledger line, because each run holds the ledger lock across its own write", func() {
			root := GinkgoT().TempDir()
			ledger := filepath.Join(root, "ledger", "runs.jsonl")
			done := make(chan error, 1)
			restore := evalcensus.FailFirstWriteTo(ledger, 3, func() {
				go func() {
					done <- evalcensus.AppendRun(ledger, []byte("{\"other\":1}\n"), root, "other", []evalcensus.Record{recordIn("other")})
				}()
				time.Sleep(200 * time.Millisecond)
			})

			err := evalcensus.AppendRun(ledger, []byte("{\"run\":1}\n"), root, "pilot", []evalcensus.Record{automatedRecord("run", `"agree"`)})
			Expect(<-done).To(Succeed())
			restore()

			Expect(err).To(HaveOccurred())
			Expect(readFile(ledger)).To(Equal("{\"other\":1}\n"))
		})
	})

	DescribeTable("a run whose records do not belong to its corpus writes nothing",
		func(corpusID string, records []evalcensus.Record) {
			base := GinkgoT().TempDir()
			root, ledger := filepath.Join(base, "census"), filepath.Join(base, "ledger", "runs.jsonl")

			err := evalcensus.AppendRun(ledger, []byte("{}\n"), root, corpusID, records)

			Expect(err).To(HaveOccurred())
			Expect(ledger).NotTo(BeAnExistingFile())
			entries, readErr := os.ReadDir(base)
			Expect(readErr).NotTo(HaveOccurred())
			for _, entry := range entries {
				Expect(entry.Name()).To(Equal("ledger"), "nothing is written for the corpus, inside the census root or out of it")
			}
		},
		Entry("a record that names another corpus", "pilot", []evalcensus.Record{recordIn("other")}),
		Entry("a corpus_id of ..", "..", []evalcensus.Record{recordIn("..")}),
		Entry("a corpus_id holding a slash", "a/b", []evalcensus.Record{recordIn("a/b")}),
		Entry("an empty corpus_id", "", []evalcensus.Record{recordIn("")}),
	)
})

// recordIn is a valid automated record that belongs to corpusID.
func recordIn(corpusID string) evalcensus.Record {
	record := automatedRecord("r-"+corpusID, `"agree"`)
	record.Target.CorpusID = corpusID
	return record
}
