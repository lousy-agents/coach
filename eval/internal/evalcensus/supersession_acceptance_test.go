package evalcensus_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

var _ = Describe("a record for a target the ledger already holds", func() {
	recordsOf := func(root string) []evalcensus.Record {
		records, err := evalcensus.LoadRecords(filepath.Join(root, "pilot", "records.jsonl"))
		Expect(err).NotTo(HaveOccurred())
		return records
	}

	It("supersedes the earlier record for the same target, and leaves that record as it was", func() {
		root := GinkgoT().TempDir()
		Expect(evalcensus.AppendCorpusRecords(root, "pilot", []evalcensus.Record{automatedRecord("first", `"agree"`)})).To(Succeed())

		Expect(evalcensus.AppendCorpusRecords(root, "pilot", []evalcensus.Record{automatedRecord("second", `"disagree"`)})).To(Succeed())

		records := recordsOf(root)
		Expect(records).To(HaveLen(2))
		Expect(records[0].Supersedes).To(BeNil())
		Expect(records[1].Supersedes).To(Equal(strPtr("first")))
	})

	It("supersedes the latest record still current, so a later re-run replaces the one before it", func() {
		root := GinkgoT().TempDir()
		Expect(evalcensus.AppendCorpusRecords(root, "pilot", []evalcensus.Record{automatedRecord("first", `"agree"`)})).To(Succeed())
		Expect(evalcensus.AppendCorpusRecords(root, "pilot", []evalcensus.Record{automatedRecord("second", `"agree"`)})).To(Succeed())

		Expect(evalcensus.AppendCorpusRecords(root, "pilot", []evalcensus.Record{automatedRecord("third", `"agree"`)})).To(Succeed())

		records := recordsOf(root)
		Expect(records).To(HaveLen(3))
		Expect(records[2].Supersedes).To(Equal(strPtr("second")))
	})

	DescribeTable("is not superseded by a record that differs in the target or the measure",
		func(other evalcensus.Record) {
			root := GinkgoT().TempDir()
			Expect(evalcensus.AppendCorpusRecords(root, "pilot", []evalcensus.Record{automatedRecord("first", `"agree"`)})).To(Succeed())

			Expect(evalcensus.AppendCorpusRecords(root, "pilot", []evalcensus.Record{other})).To(Succeed())

			Expect(recordsOf(root)[1].Supersedes).To(BeNil())
		},
		Entry("another path", func() evalcensus.Record {
			record := automatedRecord("second", `"agree"`)
			record.Target.Path = "pkg/b/b.go"
			return record
		}()),
		Entry("another measure on the same target", func() evalcensus.Record {
			record := humanRecord("second", `"true_positive"`)
			record.Target = automatedRecord("second", `"agree"`).Target
			return record
		}()),
		Entry("another SHA", func() evalcensus.Record {
			record := automatedRecord("second", `"agree"`)
			record.Target.SHA = "1111111111111111111111111111111111111111"
			return record
		}()),
	)
})

var _ = Describe("the records a reader counts", func() {
	It("counts only the records no later record supersedes", func() {
		root := GinkgoT().TempDir()
		Expect(evalcensus.AppendCorpusRecords(root, "pilot", []evalcensus.Record{automatedRecord("first", `"agree"`)})).To(Succeed())
		Expect(evalcensus.AppendCorpusRecords(root, "pilot", []evalcensus.Record{automatedRecord("second", `"agree"`)})).To(Succeed())
		Expect(evalcensus.AppendCorpusRecords(root, "pilot", []evalcensus.Record{automatedRecord("third", `"disagree"`)})).To(Succeed())
		other := automatedRecord("fourth", `"agree"`)
		other.Target.Path = "pkg/b/b.go"
		Expect(evalcensus.AppendCorpusRecords(root, "pilot", []evalcensus.Record{other})).To(Succeed())
		records, err := evalcensus.LoadRecords(filepath.Join(root, "pilot", "records.jsonl"))
		Expect(err).NotTo(HaveOccurred())

		current := evalcensus.CurrentRecords(records)

		Expect(current).To(HaveLen(2))
		Expect(current[0].RecordID).To(Equal("third"))
		Expect(current[1].RecordID).To(Equal("fourth"))
	})
})
