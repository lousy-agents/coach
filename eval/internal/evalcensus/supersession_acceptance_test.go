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

	DescribeTable("is not superseded by a record that differs in the target, the measure or the source kind",
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
		Entry("another source kind on the same target and measure", func() evalcensus.Record {
			record := automatedRecord("second", `"agree"`)
			record.Source = evalcensus.Source{Kind: "census", Ref: "gocognit v2.0.0"}
			return record
		}()),
		Entry("only the measure, from the same source", func() evalcensus.Record {
			record := automatedRecord("second", `1`)
			record.Measure = "flagged_vs_unflagged_association"
			return record
		}()),
	)

	It("does not supersede a record that an earlier correction already supersedes", func() {
		root := GinkgoT().TempDir()
		correction := automatedRecord("correction", `"disagree"`)
		correction.Source = evalcensus.Source{Kind: "census", Ref: "gocognit v2.0.0"}
		correction.Supersedes = strPtr("first")
		Expect(evalcensus.AppendCorpusRecords(root, "pilot", []evalcensus.Record{automatedRecord("first", `"agree"`)})).To(Succeed())
		Expect(evalcensus.AppendCorpusRecords(root, "pilot", []evalcensus.Record{correction})).To(Succeed())

		Expect(evalcensus.AppendCorpusRecords(root, "pilot", []evalcensus.Record{automatedRecord("third", `"agree"`)})).To(Succeed())

		records := recordsOf(root)
		Expect(records).To(HaveLen(3))
		Expect(records[2].Supersedes).To(BeNil(), "the first record is already superseded, and the correction is from another source kind")
	})

	It("supersedes the latest of two current records for one target, and leaves the earlier one as it was", func() {
		path := writeLedger(recordLine(automatedRecord("earlier", `"agree"`)) + "\n" + recordLine(automatedRecord("later", `"agree"`)) + "\n")

		Expect(evalcensus.AppendRecords(path, []evalcensus.Record{automatedRecord("third", `"agree"`)})).To(Succeed())

		records, err := evalcensus.LoadRecords(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(records).To(HaveLen(3))
		Expect(records[0].Supersedes).To(BeNil())
		Expect(records[1].Supersedes).To(BeNil())
		Expect(records[2].Supersedes).To(Equal(strPtr("later")))
	})

	It("supersedes the earlier record when a history re-run moves its association from none to a fix", func() {
		root := GinkgoT().TempDir()
		earlier := automatedRecord("earlier", `0`)
		earlier.Measure = "flagged_vs_unflagged_association"
		earlier.Source = evalcensus.Source{Kind: "szz", Ref: "none"}
		later := automatedRecord("later", `1`)
		later.Measure = "flagged_vs_unflagged_association"
		later.Source = evalcensus.Source{Kind: "szz", Ref: "abc1234"}
		Expect(evalcensus.AppendCorpusRecords(root, "pilot", []evalcensus.Record{earlier})).To(Succeed())

		Expect(evalcensus.AppendCorpusRecords(root, "pilot", []evalcensus.Record{later})).To(Succeed())

		Expect(recordsOf(root)[1].Supersedes).To(Equal(strPtr("earlier")))
	})

	It("supersedes the earlier census record when a Coach change re-measures the same target", func() {
		root := GinkgoT().TempDir()
		earlier := automatedRecord("earlier", `"measured"`)
		earlier.Target.Path = "."
		earlier.Target.Lines = nil
		earlier.Measure = "analysis_status"
		earlier.Source = evalcensus.Source{Kind: "census", Ref: "coach-hash-a"}
		later := earlier
		later.RecordID = "later"
		later.Source = evalcensus.Source{Kind: "census", Ref: "coach-hash-b"}
		Expect(evalcensus.AppendCorpusRecords(root, "pilot", []evalcensus.Record{earlier})).To(Succeed())

		Expect(evalcensus.AppendCorpusRecords(root, "pilot", []evalcensus.Record{later})).To(Succeed())

		Expect(recordsOf(root)[1].Supersedes).To(Equal(strPtr("earlier")))
	})

	It("lets only the first of two records in one batch supersede the earlier one", func() {
		root := GinkgoT().TempDir()
		Expect(evalcensus.AppendCorpusRecords(root, "pilot", []evalcensus.Record{automatedRecord("first", `"agree"`)})).To(Succeed())

		Expect(evalcensus.AppendCorpusRecords(root, "pilot", []evalcensus.Record{automatedRecord("second", `"agree"`), automatedRecord("third", `"agree"`)})).To(Succeed())

		records := recordsOf(root)
		Expect(records).To(HaveLen(3))
		Expect(records[1].Supersedes).To(Equal(strPtr("first")))
		Expect(records[2].Supersedes).To(BeNil())
	})
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
