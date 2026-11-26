package evalgrade_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const reachChannel = `const channel = "/proc/" + process.ppid + "/fd/4";` + "\n"

var _ = Describe("a test that reaches the reporter's record channel by another route (781 AC-3, design 7)", func() {
	It("adds a record without the nonce and a line that is not JSON, and neither counts as a result", func() {
		run := runFixture(map[string]string{"a.test.ts": header + `
test("forges through the parent", () => {
  ` + reachChannel + `
  fs.appendFileSync(channel, JSON.stringify({ nonce: "guess", id: "a.test.ts::never ran", status: "pass" }) + "\n{not json\n");
});
`})

		Expect(statusOf(run, "a.test.ts::never ran")).To(BeEmpty())
		Expect(statusOf(run, "a.test.ts::forges through the parent")).To(Equal([]string{"pass"}))
		Expect(run.Records).To(ContainElement(record("guess", "a.test.ts::never ran", "pass")), "kept so that the verifier can report it as forged")
		Expect(run.Malformed).To(Equal(1))
	})

	It("leaves a line cut short, which swallows that test's own record: it goes missing, never a pass, and the next test is read", func() {
		run := runFixture(map[string]string{"a.test.ts": header + `
test("cuts a line", () => {
  ` + reachChannel + `
  fs.appendFileSync(channel, "{\"nonce\":");
});
test("comes after", () => {});
`})

		Expect(statusOf(run, "a.test.ts::cuts a line")).To(BeEmpty())
		Expect(statusOf(run, "a.test.ts::comes after")).To(Equal([]string{"pass"}))
		Expect(run.Malformed).To(Equal(1))
	})

	It("survives a line far longer than any record, and keeps the records after it", func() {
		run := runFixture(map[string]string{"a.test.ts": header + `
test("floods", () => {
  ` + reachChannel + `
  fs.appendFileSync(channel, "x".repeat(4 * 1024 * 1024) + "\n");
});
test("comes after the flood", () => {});
`})

		Expect(statusOf(run, "a.test.ts::comes after the flood")).To(Equal([]string{"pass"}))
		Expect(run.Malformed).To(Equal(1))
	})
})
