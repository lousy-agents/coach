package evalcensus_test

import (
	"encoding/json"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

const fileLevelRecord = `{"record_id":"f1","target":{"corpus_id":"pilot","sha":"` + fixtureSHA + `","rule_id":"complexity.branch_density","path":"pkg/a/a.go","subject":null,"lines":null,"fingerprint":null},"measure":"detector_agreement","value":"agree","source":{"kind":"detector","ref":"gocyclo"},"rater":{"id":null,"kind":"automated"},"date":"2026-11-26","rationale":"","disposition":null,"engineer_action":null,"supersedes":null}`

var _ = Describe("the census rulings", func() {
	When("a finding is file-level, with no line span", func() {
		It("records it with lines null and reads it back", func() {
			var record evalcensus.Record
			Expect(json.Unmarshal([]byte(fileLevelRecord), &record)).To(Succeed())
			path := ledgerPath()

			Expect(evalcensus.AppendRecords(path, []evalcensus.Record{record})).To(Succeed())

			Expect(readFile(path)).To(ContainSubstring(`"lines":null`))
			records, err := evalcensus.LoadRecords(path)
			Expect(err).NotTo(HaveOccurred())
			Expect(records).To(HaveLen(1))
		})
	})

	DescribeTable("a measure takes only its own kind of value",
		func(measure, value string, allowed bool) {
			record := automatedRecord("r1", value)
			record.Measure = measure

			err := evalcensus.AppendRecords(ledgerPath(), []evalcensus.Record{record})
			if allowed {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(MatchError(evalcensus.ErrInvalidRecord))
			}
		},
		Entry("an agreement of agree", "detector_agreement", `"agree"`, true),
		Entry("an agreement of disagree", "detector_agreement", `"disagree"`, true),
		Entry("an agreement given as a number", "detector_agreement", `0.4`, false),
		Entry("an association given as a number", "flagged_vs_unflagged_association", `0.4`, true),
		Entry("an association of agree", "flagged_vs_unflagged_association", `"agree"`, false),
		Entry("an association of disagree", "flagged_vs_unflagged_association", `"disagree"`, false),
	)

	When("a write stops part way through", func() {
		It("truncates the ledger back to its length before the append", func() {
			path := ledgerPath()
			Expect(evalcensus.AppendRecords(path, []evalcensus.Record{automatedRecord("r1", `"agree"`)})).To(Succeed())
			before := readFile(path)

			restore := evalcensus.FailLedgerWritesAfter(20)
			err := evalcensus.AppendRecords(path, []evalcensus.Record{automatedRecord("r2", `"agree"`)})
			restore()

			Expect(err).To(HaveOccurred())
			Expect(readFile(path)).To(Equal(before))
			Expect(evalcensus.AppendRecords(path, []evalcensus.Record{automatedRecord("r2", `"agree"`)})).To(Succeed())
			_, statErr := os.Stat(path + ".lock")
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "the lock is released after a failed write")
		})
	})

	When("record ids are generated", func() {
		It("gives the same content a shared hash prefix and a fresh random suffix each time", func() {
			record := automatedRecord("", `"agree"`)
			first, second := evalcensus.NewRecordID(record), evalcensus.NewRecordID(record)
			other := automatedRecord("", `"disagree"`)

			Expect(first).NotTo(Equal(second))
			Expect(first).To(MatchRegexp(`^[0-9a-f]{16}-[0-9a-f]{16}$`))
			Expect(strings.Split(first, "-")[0]).To(Equal(strings.Split(second, "-")[0]))
			Expect(strings.Split(evalcensus.NewRecordID(other), "-")[0]).NotTo(Equal(strings.Split(first, "-")[0]))

			named := automatedRecord("already-named", `"agree"`)
			Expect(strings.Split(evalcensus.NewRecordID(named), "-")[0]).To(Equal(strings.Split(first, "-")[0]), "the hash ignores any record_id the record carries")
		})
	})
})
