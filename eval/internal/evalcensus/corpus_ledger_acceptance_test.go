package evalcensus_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

var _ = Describe("a corpus's census ledger", func() {
	When("records for the corpus are appended", func() {
		It("writes them to <census root>/<corpus_id>/records.jsonl", func() {
			root := GinkgoT().TempDir()

			Expect(evalcensus.AppendCorpusRecords(root, "pilot", []evalcensus.Record{automatedRecord("r1", `"agree"`)})).To(Succeed())

			records, err := evalcensus.LoadRecords(filepath.Join(root, "pilot", "records.jsonl"))
			Expect(err).NotTo(HaveOccurred())
			Expect(evalcensus.CurrentRecords(records)).To(HaveLen(1))
		})
	})

	When("a record names another corpus", func() {
		It("refuses the batch", func() {
			record := automatedRecord("r1", `"agree"`)
			record.Target.CorpusID = "other"

			Expect(evalcensus.AppendCorpusRecords(GinkgoT().TempDir(), "pilot", []evalcensus.Record{record})).To(MatchError(evalcensus.ErrInvalidRecord))
		})
	})

	When("a later record in the batch names another corpus", func() {
		It("refuses the whole batch and writes nothing", func() {
			root := GinkgoT().TempDir()
			stray := automatedRecord("r2", `"agree"`)
			stray.Target.CorpusID = "other"

			err := evalcensus.AppendCorpusRecords(root, "pilot", []evalcensus.Record{automatedRecord("r1", `"agree"`), stray})
			Expect(err).To(MatchError(evalcensus.ErrInvalidRecord))
			Expect(filepath.Join(root, "pilot", "records.jsonl")).NotTo(BeAnExistingFile())
		})
	})

	DescribeTable("a corpus_id that is not a path segment never becomes a path",
		func(corpusID string) {
			Expect(evalcensus.AppendCorpusRecords(GinkgoT().TempDir(), corpusID, nil)).To(MatchError(ContainSubstring("corpus_id")))
		},
		Entry("..", ".."),
		Entry("a slash", "a/b"),
		Entry("empty", ""),
	)
})
