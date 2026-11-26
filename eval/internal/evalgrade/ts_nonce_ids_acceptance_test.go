package evalgrade_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalgrade"
)

func idsOf(run evalgrade.TypeScriptRun) []string {
	ids := []string{}
	for _, record := range run.Records {
		ids = append(ids, record.ID)
	}
	return ids
}

var _ = Describe("test IDs when describe blocks nest and repeat (reviewer block on #850)", func() {
	It("names each test by its own chain when two sibling describes sit under one parent, and a flat test follows", func() {
		run := runFixture(map[string]string{"a.test.ts": header + `
describe("outer", () => {
  describe("first", () => { test("a", () => {}); });
  describe("second", () => { test("b", () => {}); });
  test("direct", () => {});
});
test("after", () => {});
`})

		Expect(idsOf(run)).To(ConsistOf(
			"a.test.ts::outer > first > a",
			"a.test.ts::outer > second > b",
			"a.test.ts::outer > direct",
			"a.test.ts::after",
		))
	})
})

var _ = Describe("what the harness will not accept as a nonce, and the flags it runs node with (reviewer block on #850)", func() {
	DescribeTable("refuses a nonce the reporter would read back differently",
		func(nonce string) {
			dir, names := tsFixture(map[string]string{"a.test.ts": header + "test(\"one\", () => {});\n"})

			_, err := evalgrade.RunTypeScriptTests(nodePath(), dir, names, nonce)

			Expect(err).To(MatchError(ContainSubstring("nonce")))
		},
		Entry("a carriage return alone", "ab\rcd"),
		Entry("a leading space", " n-1"),
		Entry("a trailing space", "n-1 "),
		Entry("a trailing tab", "n-1\t"),
	)

	It("runs every test file in its own process, whatever Node's default becomes", func() {
		dir := GinkgoT().TempDir()
		fake := filepath.Join(dir, "fake-node")
		Expect(os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > argv.txt\n"), 0o755)).To(Succeed())

		_, err := evalgrade.RunTypeScriptTests(fake, dir, []string{"a.test.ts"}, testNonce)

		Expect(err).NotTo(HaveOccurred())
		argv, readErr := os.ReadFile(filepath.Join(dir, "argv.txt"))
		Expect(readErr).NotTo(HaveOccurred())
		Expect(string(argv)).To(ContainSubstring("--test-isolation=process\n"))
		Expect(string(argv)).To(ContainSubstring("--test\n"))
	})

	It("lists unexpected IDs in a fixed order", func() {
		verdicts := evalgrade.Verify([]evalgrade.TestRecord{
			record(testNonce, "z.test.ts::z", "pass"), record(testNonce, "a.test.ts::a", "pass"), record(testNonce, "m.test.ts::m", "pass"),
		}, testNonce, []string{"a.test.ts::one"})

		Expect(verdicts.Unexpected).To(Equal([]string{"a.test.ts::a", "m.test.ts::m", "z.test.ts::z"}))
	})
})
