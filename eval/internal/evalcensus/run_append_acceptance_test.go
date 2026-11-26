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
})
