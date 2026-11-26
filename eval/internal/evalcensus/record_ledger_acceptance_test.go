package evalcensus_test

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalcensus"
)

var _ = Describe("the census record ledger", func() {
	When("valid records are appended", func() {
		It("writes one JSON line per record and reads them back", func() {
			path := ledgerPath()

			Expect(evalcensus.AppendRecords(path, []evalcensus.Record{automatedRecord("r1", `"agree"`), humanRecord("r2", `"false_positive"`)})).To(Succeed())

			records, err := evalcensus.LoadRecords(path)
			Expect(err).NotTo(HaveOccurred())
			Expect(records).To(HaveLen(2))
			Expect(records[1].RecordID).To(Equal("r2"))
		})
	})

	When("a later batch is appended", func() {
		It("leaves every earlier line byte for byte", func() {
			path := ledgerPath()
			Expect(evalcensus.AppendRecords(path, []evalcensus.Record{automatedRecord("r1", `"agree"`)})).To(Succeed())
			before := readFile(path)

			Expect(evalcensus.AppendRecords(path, []evalcensus.Record{automatedRecord("r2", `0.42`)})).To(Succeed())

			Expect(readFile(path)).To(HavePrefix(before))
		})
	})

	DescribeTable("an automated source may write only agree, disagree or a number",
		func(value string, allowed bool) {
			err := evalcensus.AppendRecords(ledgerPath(), []evalcensus.Record{automatedRecord("r1", value)})
			if allowed {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(MatchError(evalcensus.ErrInvalidRecord))
			}
		},
		Entry("agree", `"agree"`, true),
		Entry("disagree", `"disagree"`, true),
		Entry("a number", `0.37`, true),
		Entry("true_positive", `"true_positive"`, false),
		Entry("false_positive", `"false_positive"`, false),
		Entry("unactionable_true_positive", `"unactionable_true_positive"`, false),
		Entry("not_adjudicable", `"not_adjudicable"`, false),
		Entry("another string", `"maybe"`, false),
		Entry("null", `null`, false),
	)

	DescribeTable("every automated source kind is held to that rule",
		func(sourceKind string) {
			record := automatedRecord("r1", `"true_positive"`)
			record.Source.Kind = sourceKind

			Expect(evalcensus.AppendRecords(ledgerPath(), []evalcensus.Record{record})).To(MatchError(evalcensus.ErrInvalidRecord))
		},
		Entry("detector", "detector"),
		Entry("szz", "szz"),
		Entry("review_comment", "review_comment"),
		Entry("follow_up_fix", "follow_up_fix"),
	)

	DescribeTable("a record that is not a whole human verdict is refused",
		func(mutate func(*evalcensus.Record)) {
			record := humanRecord("r1", `"true_positive"`)
			mutate(&record)

			Expect(evalcensus.AppendRecords(ledgerPath(), []evalcensus.Record{record})).To(MatchError(evalcensus.ErrInvalidRecord))
		},
		Entry("a verdict value from an automated rater", func(r *evalcensus.Record) { r.Rater = evalcensus.Rater{Kind: "automated"} }),
		Entry("a verdict value from an automated source", func(r *evalcensus.Record) { r.Source.Kind = "detector" }),
		Entry("a human verdict with no rater name", func(r *evalcensus.Record) { r.Rater.ID = nil }),
		Entry("a human verdict with no rationale", func(r *evalcensus.Record) { r.Rationale = "" }),
		Entry("a human rater writing an automated measure", func(r *evalcensus.Record) { r.Measure = "detector_agreement" }),
		Entry("a human verdict that is agree", func(r *evalcensus.Record) { r.Value = json.RawMessage(`"agree"`) }),
	)

	DescribeTable("a record with a malformed field is refused",
		func(mutate func(*evalcensus.Record)) {
			record := automatedRecord("r1", `"agree"`)
			mutate(&record)

			Expect(evalcensus.AppendRecords(ledgerPath(), []evalcensus.Record{record})).To(MatchError(evalcensus.ErrInvalidRecord))
		},
		Entry("an empty record_id", func(r *evalcensus.Record) { r.RecordID = "" }),
		Entry("an empty corpus_id", func(r *evalcensus.Record) { r.Target.CorpusID = "" }),
		Entry("a SHA that is not 40 hex digits", func(r *evalcensus.Record) { r.Target.SHA = "HEAD" }),
		Entry("an empty rule_id", func(r *evalcensus.Record) { r.Target.RuleID = "" }),
		Entry("an empty path", func(r *evalcensus.Record) { r.Target.Path = "" }),
		Entry("a line range that ends before it starts", func(r *evalcensus.Record) { r.Target.Lines = [2]int{9, 3} }),
		Entry("a line range starting at 0", func(r *evalcensus.Record) { r.Target.Lines = [2]int{0, 3} }),
		Entry("an unknown measure", func(r *evalcensus.Record) { r.Measure = "vibes" }),
		Entry("an unknown source kind", func(r *evalcensus.Record) { r.Source.Kind = "rumour" }),
		Entry("an unknown rater kind", func(r *evalcensus.Record) { r.Rater.Kind = "robot" }),
		Entry("a date that is not YYYY-MM-DD", func(r *evalcensus.Record) { r.Date = "26/11/2026" }),
		Entry("an unknown disposition", func(r *evalcensus.Record) { r.Disposition = strPtr("ignore") }),
		Entry("an unknown engineer_action", func(r *evalcensus.Record) { r.EngineerAction = strPtr("shrugged") }),
	)

	When("a batch holds one invalid record", func() {
		It("writes none of the batch", func() {
			path := ledgerPath()

			err := evalcensus.AppendRecords(path, []evalcensus.Record{automatedRecord("r1", `"agree"`), automatedRecord("r2", `"true_positive"`)})
			Expect(err).To(MatchError(evalcensus.ErrInvalidRecord))

			_, statErr := os.Stat(path)
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "an invalid batch must leave no ledger behind")
		})
	})

	When("a record_id is already in the ledger or the batch", func() {
		It("refuses, because records are never rewritten", func() {
			path := ledgerPath()
			Expect(evalcensus.AppendRecords(path, []evalcensus.Record{automatedRecord("r1", `"agree"`)})).To(Succeed())

			Expect(evalcensus.AppendRecords(path, []evalcensus.Record{automatedRecord("r1", `"disagree"`)})).To(MatchError(evalcensus.ErrInvalidRecord))
			Expect(evalcensus.AppendRecords(path, []evalcensus.Record{automatedRecord("r2", `"agree"`), automatedRecord("r2", `"agree"`)})).To(MatchError(evalcensus.ErrInvalidRecord))
		})
	})

	When("a record supersedes another", func() {
		It("accepts an earlier record_id and refuses an unknown one", func() {
			path := ledgerPath()
			Expect(evalcensus.AppendRecords(path, []evalcensus.Record{automatedRecord("r1", `"agree"`)})).To(Succeed())

			correction := automatedRecord("r2", `"disagree"`)
			correction.Supersedes = strPtr("r1")
			Expect(evalcensus.AppendRecords(path, []evalcensus.Record{correction})).To(Succeed())

			dangling := automatedRecord("r3", `"agree"`)
			dangling.Supersedes = strPtr("r9")
			Expect(evalcensus.AppendRecords(path, []evalcensus.Record{dangling})).To(MatchError(evalcensus.ErrInvalidRecord))
		})
	})

	When("the existing ledger holds a line that is not a valid record", func() {
		It("refuses to append to it", func() {
			path := ledgerPath()
			Expect(os.MkdirAll(filepath.Dir(path), 0o755)).To(Succeed())
			Expect(os.WriteFile(path, []byte("{\"record_id\":\"r1\"}\n"), 0o644)).To(Succeed())

			Expect(evalcensus.AppendRecords(path, []evalcensus.Record{automatedRecord("r2", `"agree"`)})).To(MatchError(ContainSubstring("line 1")))
		})
	})
})
