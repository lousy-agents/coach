package evalcensus_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

func recordLine(record evalcensus.Record) string {
	raw, err := json.Marshal(record)
	Expect(err).NotTo(HaveOccurred())
	return string(raw)
}

func writeLedger(content string) string {
	path := ledgerPath()
	Expect(os.MkdirAll(filepath.Dir(path), 0o755)).To(Succeed())
	Expect(os.WriteFile(path, []byte(content), 0o644)).To(Succeed())
	return path
}

func withSupersedes(record evalcensus.Record, id string) evalcensus.Record {
	record.Supersedes = strPtr(id)
	return record
}

var _ = Describe("the census ledger's integrity", func() {
	When("the ledger's last line has no newline", func() {
		It("still appends the new record as its own line", func() {
			path := writeLedger(recordLine(automatedRecord("r1", `"agree"`)))

			Expect(evalcensus.AppendRecords(path, []evalcensus.Record{automatedRecord("r2", `"agree"`)})).To(Succeed())

			records, err := evalcensus.LoadRecords(path)
			Expect(err).NotTo(HaveOccurred())
			Expect(records).To(HaveLen(2))
		})
	})

	When("one ledger line holds two records", func() {
		It("refuses the ledger rather than reading only the first", func() {
			path := writeLedger(recordLine(automatedRecord("r1", `"agree"`)) + recordLine(automatedRecord("r2", `"agree"`)) + "\n")

			_, err := evalcensus.LoadRecords(path)
			Expect(err).To(MatchError(ContainSubstring("line 1")))
		})
	})

	DescribeTable("an existing ledger that breaks a cross-record rule is refused",
		func(lines ...evalcensus.Record) {
			var content strings.Builder
			for _, record := range lines {
				content.WriteString(recordLine(record) + "\n")
			}
			path := writeLedger(content.String())

			_, loadErr := evalcensus.LoadRecords(path)
			Expect(loadErr).To(MatchError(evalcensus.ErrInvalidRecord))
			Expect(evalcensus.AppendRecords(path, []evalcensus.Record{automatedRecord("r9", `"agree"`)})).To(MatchError(evalcensus.ErrInvalidRecord))
		},
		Entry("a record_id used twice", automatedRecord("r1", `"agree"`), automatedRecord("r1", `"disagree"`)),
		Entry("a supersedes that names no earlier record", automatedRecord("r1", `"agree"`), withSupersedes(automatedRecord("r2", `"agree"`), "r7")),
	)

	When("several writers append the same record_id at once", func() {
		It("lets exactly one of them write it", func() {
			path := ledgerPath()
			var wg sync.WaitGroup
			for i := 0; i < 8; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					defer GinkgoRecover()
					_ = evalcensus.AppendRecords(path, []evalcensus.Record{automatedRecord("same", `"agree"`)})
				}()
			}
			wg.Wait()

			records, err := evalcensus.LoadRecords(path)
			Expect(err).NotTo(HaveOccurred())
			Expect(evalcensus.CurrentRecords(records)).To(HaveLen(1))
		})
	})
})
