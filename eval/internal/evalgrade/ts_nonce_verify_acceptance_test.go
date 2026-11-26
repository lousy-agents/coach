package evalgrade_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalgrade"
)

func record(nonce, id, status string) evalgrade.TestRecord {
	return evalgrade.TestRecord{Nonce: nonce, ID: id, Status: status}
}

var _ = Describe("G1 verification of the nonce channel's records (design 4, 7)", func() {
	ids := []string{"a.test.ts::one", "a.test.ts::two"}

	It("gives each expected ID the status of its one record with the nonce", func() {
		verdicts := evalgrade.Verify([]evalgrade.TestRecord{
			record(testNonce, "a.test.ts::one", "pass"), record(testNonce, "a.test.ts::two", "fail"),
		}, testNonce, ids)

		Expect(verdicts.ByID).To(Equal(map[string]evalgrade.Verdict{"a.test.ts::one": evalgrade.VerdictPass, "a.test.ts::two": evalgrade.VerdictFail}))
		Expect(verdicts.AllPass()).To(BeFalse())
	})

	It("passes only when every expected ID has a passing record", func() {
		verdicts := evalgrade.Verify([]evalgrade.TestRecord{
			record(testNonce, "a.test.ts::one", "pass"), record(testNonce, "a.test.ts::two", "pass"),
		}, testNonce, ids)

		Expect(verdicts.AllPass()).To(BeTrue())
	})

	It("reports an expected ID with no record as missing", func() {
		verdicts := evalgrade.Verify([]evalgrade.TestRecord{record(testNonce, "a.test.ts::one", "pass")}, testNonce, ids)

		Expect(verdicts.ByID["a.test.ts::two"]).To(Equal(evalgrade.VerdictMissing))
		Expect(verdicts.AllPass()).To(BeFalse())
	})

	It("counts no record with another nonce, and reports it as forged", func() {
		verdicts := evalgrade.Verify([]evalgrade.TestRecord{
			record("other", "a.test.ts::one", "pass"), record(testNonce, "a.test.ts::two", "pass"),
		}, testNonce, ids)

		Expect(verdicts.ByID["a.test.ts::one"]).To(Equal(evalgrade.VerdictMissing))
		Expect(verdicts.Forged).To(Equal(1))
	})

	It("keeps the status of the real record when a forged one claims the same ID", func() {
		verdicts := evalgrade.Verify([]evalgrade.TestRecord{
			record(testNonce, "a.test.ts::one", "fail"), record("", "a.test.ts::one", "pass"), record(testNonce, "a.test.ts::two", "pass"),
		}, testNonce, ids)

		Expect(verdicts.ByID["a.test.ts::one"]).To(Equal(evalgrade.VerdictFail))
		Expect(verdicts.Forged).To(Equal(1))
	})

	It("refuses an ID that two records with the nonce both claim", func() {
		verdicts := evalgrade.Verify([]evalgrade.TestRecord{
			record(testNonce, "a.test.ts::one", "pass"), record(testNonce, "a.test.ts::one", "pass"), record(testNonce, "a.test.ts::two", "pass"),
		}, testNonce, ids)

		Expect(verdicts.ByID["a.test.ts::one"]).To(Equal(evalgrade.VerdictDuplicate))
		Expect(verdicts.AllPass()).To(BeFalse())
	})

	It("lists a record with the nonce for an ID nobody expected, and does not count it", func() {
		verdicts := evalgrade.Verify([]evalgrade.TestRecord{
			record(testNonce, "a.test.ts::one", "pass"), record(testNonce, "a.test.ts::two", "pass"), record(testNonce, "a.test.ts::extra", "pass"),
		}, testNonce, ids)

		Expect(verdicts.Unexpected).To(Equal([]string{"a.test.ts::extra"}))
		Expect(verdicts.AllPass()).To(BeTrue(), "an extra record does not fail the expected IDs; the caller reads Unexpected")
	})

	It("accepts nothing when the nonce it was given is empty, whatever the records carry", func() {
		verdicts := evalgrade.Verify([]evalgrade.TestRecord{
			record("", "a.test.ts::one", "pass"), record("", "a.test.ts::two", "pass"),
		}, "", ids)

		Expect(verdicts.AllPass()).To(BeFalse())
		Expect(verdicts.ByID["a.test.ts::one"]).To(Equal(evalgrade.VerdictMissing))
	})

	It("never reports an empty expectation as passing", func() {
		Expect(evalgrade.Verify(nil, testNonce, nil).AllPass()).To(BeFalse())
	})

	DescribeTable("maps each status the reporter writes",
		func(status string, want evalgrade.Verdict) {
			verdicts := evalgrade.Verify([]evalgrade.TestRecord{record(testNonce, "a.test.ts::one", status)}, testNonce, []string{"a.test.ts::one"})

			Expect(verdicts.ByID["a.test.ts::one"]).To(Equal(want))
		},
		Entry("pass", "pass", evalgrade.VerdictPass),
		Entry("fail", "fail", evalgrade.VerdictFail),
		Entry("skipped", "skipped", evalgrade.VerdictSkipped),
		Entry("todo", "todo", evalgrade.VerdictTodo),
		Entry("a status nobody defined", "ok", evalgrade.VerdictFail),
	)
})
