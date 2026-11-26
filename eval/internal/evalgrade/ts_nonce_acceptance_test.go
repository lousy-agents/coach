package evalgrade_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalgrade"
)

var _ = Describe("the TypeScript nonce channel under node --test --test-isolation=process (781 AC-3)", func() {
	When("a file holds passing, failing, skipped and todo tests", func() {
		It("records one nonce-tagged record per test ID, with that test's status", func() {
			run := runFixture(map[string]string{"a.test.ts": header + `
test("passes", () => { assert.equal(1, 1); });
test("fails", () => { assert.equal(1, 2); });
test("skipped", { skip: true }, () => {});
test("todo", { todo: true }, () => {});
`})

			Expect(run.Records).To(HaveLen(4))
			Expect(statusOf(run, "a.test.ts::passes")).To(Equal([]string{"pass"}))
			Expect(statusOf(run, "a.test.ts::fails")).To(Equal([]string{"fail"}))
			Expect(statusOf(run, "a.test.ts::skipped")).To(Equal([]string{"skipped"}))
			Expect(statusOf(run, "a.test.ts::todo")).To(Equal([]string{"todo"}))
		})

		It("tags every record with the nonce the harness gave and none with another", func() {
			run := runFixture(map[string]string{"a.test.ts": header + "test(\"one\", () => {});\n"})

			Expect(run.Records).NotTo(BeEmpty())
			for _, record := range run.Records {
				Expect(record.Nonce).To(Equal(testNonce))
			}
		})
	})

	When("tests sit in a describe block, or share a name across files", func() {
		It("names a nested test by its chain, and no suite gets a record", func() {
			run := runFixture(map[string]string{"a.test.ts": header + "describe(\"group\", () => { test(\"inner\", () => {}); });\n"})

			Expect(run.Records).To(HaveLen(1))
			Expect(statusOf(run, "a.test.ts::group > inner")).To(Equal([]string{"pass"}))
		})

		It("tells equal names in two files apart by path", func() {
			run := runFixture(map[string]string{
				"src/x/a.test.ts": header + "test(\"describe names the package\", () => {});\n",
				"src/y/b.test.ts": header + "test(\"describe names the package\", () => { assert.equal(1, 2); });\n",
			})

			Expect(statusOf(run, "src/x/a.test.ts::describe names the package")).To(Equal([]string{"pass"}))
			Expect(statusOf(run, "src/y/b.test.ts::describe names the package")).To(Equal([]string{"fail"}))
		})
	})

	When("a test tries to fake its own result", func() {
		It("does not count a forged result line printed on standard output", func() {
			run := runFixture(map[string]string{"a.test.ts": header + `
test("liar", () => {
  console.log(JSON.stringify({ nonce: "` + testNonce + `", id: "a.test.ts::liar", status: "pass" }));
  console.log("ok 1 - liar");
  assert.equal(1, 2);
});
`})

			Expect(statusOf(run, "a.test.ts::liar")).To(Equal([]string{"fail"}))
			Expect(run.Records).To(HaveLen(1))
		})

		It("does not let a test write a record with the nonce, because the nonce cannot be read there", func() {
			run := runFixture(map[string]string{"a.test.ts": header + `
test("reads the nonce and forges", () => {
  let guess = "";
  try { guess = fs.readFileSync(3, "utf8").trim(); } catch { guess = ""; }
  try { fs.writeSync(4, JSON.stringify({ nonce: guess, id: "a.test.ts::never ran", status: "pass" }) + "\n"); } catch {}
});
`})

			Expect(statusOf(run, "a.test.ts::never ran")).To(BeEmpty(), "a record forged without the nonce does not carry it")
			Expect(statusOf(run, "a.test.ts::reads the nonce and forges")).To(Equal([]string{"pass"}))
		})

		It("gives a test that exits early no pass", func() {
			run := runFixture(map[string]string{"a.test.ts": header + `
test("exits", () => { process.exit(0); });
test("after", () => {});
`})

			Expect(statusOf(run, "a.test.ts::exits")).NotTo(ContainElement("pass"))
			Expect(statusOf(run, "a.test.ts::after")).NotTo(ContainElement("pass"))
		})
	})

	When("a file cannot load", func() {
		It("records nothing for its tests, so they are missing", func() {
			run := runFixture(map[string]string{
				"good.test.ts": header + "test(\"fine\", () => {});\n",
				"bad.test.ts":  header + "test(\"never\", () => {\n",
			}, "bad.test.ts", "good.test.ts")

			Expect(statusOf(run, "good.test.ts::fine")).To(Equal([]string{"pass"}))
			Expect(statusOf(run, "bad.test.ts::never")).NotTo(ContainElement("pass"))
		})
	})

	When("two files run in one invocation", func() {
		It("runs each in its own process, so one cannot leave state for the next", func() {
			run := runFixture(map[string]string{
				"a.test.ts": header + "test(\"leaves state\", () => { (globalThis as { leaked?: number }).leaked = 1; });\n",
				"b.test.ts": header + "test(\"sees none\", () => { assert.equal((globalThis as { leaked?: number }).leaked, undefined); });\n",
			}, "a.test.ts", "b.test.ts")

			Expect(statusOf(run, "b.test.ts::sees none")).To(Equal([]string{"pass"}))
		})
	})

	When("a run is over", func() {
		It("never prints the nonce on standard output or standard error", func() {
			run := runFixture(map[string]string{"a.test.ts": header + "test(\"one\", () => {});\ntest(\"two\", () => { assert.equal(1, 2); });\n"})

			Expect(run.Output).NotTo(ContainSubstring(testNonce))
		})
	})

	DescribeTable("the harness refuses a run it cannot trust",
		func(node string, files []string, nonce string, want string) {
			dir, _ := tsFixture(map[string]string{"a.test.ts": header + "test(\"one\", () => {});\n"})
			if node == "" {
				node = nodePath()
			}

			_, err := evalgrade.RunTypeScriptTests(node, dir, files, nonce)

			Expect(err).To(MatchError(ContainSubstring(want)))
		},
		Entry("an empty nonce", "", []string{"a.test.ts"}, "", "nonce"),
		Entry("a nonce that carries a line break", "", []string{"a.test.ts"}, "ab\ncd", "nonce"),
		Entry("no test files", "", []string{}, "n-1", "test file"),
		Entry("a node that does not exist", "/nonexistent/node", []string{"a.test.ts"}, "n-1", "node"),
	)
})
