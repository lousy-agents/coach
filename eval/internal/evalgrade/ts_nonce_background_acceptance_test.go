package evalgrade_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalgrade"
)

// runWithin returns how long RunTypeScriptTestsWithin took, so a run that a leftover process holds open is a failure
// of the spec rather than a hang of the suite.
func runWithin(files map[string]string, limit time.Duration) (evalgrade.TypeScriptRun, time.Duration) {
	dir, names := tsFixture(files)
	started := time.Now()
	run, err := evalgrade.RunTypeScriptTestsWithin(nodePath(), dir, names, testNonce, limit)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return run, time.Since(started)
}

var _ = Describe("a test that leaves a background process behind (781 AC-3, reviewer block on #850)", func() {
	const bound = 30 * time.Second

	It("does not hold the run open when the process keeps the test's standard streams", func() {
		run, took := runWithin(map[string]string{"a.test.ts": header + `
import { spawn } from "node:child_process";
test("leaves a holder", () => {
  spawn("sleep", ["120"], { detached: true, stdio: "inherit" }).unref();
});
`}, 3*time.Second)

		Expect(took).To(BeNumerically("<", bound))
		Expect(statusOf(run, "a.test.ts::leaves a holder")).To(Equal([]string{"pass"}))
	})

	It("does not hold the run open when the process holds a copy of the record channel", func() {
		run, took := runWithin(map[string]string{"a.test.ts": header + `
import { spawn } from "node:child_process";
test("leaves a holder of the record fd", () => {
  const copy = fs.openSync("/proc/" + process.ppid + "/fd/4", "a");
  spawn("sleep", ["120"], { detached: true, stdio: ["ignore", "ignore", "ignore", "ignore", copy] }).unref();
});
`}, 3*time.Second)

		Expect(took).To(BeNumerically("<", bound))
		Expect(statusOf(run, "a.test.ts::leaves a holder of the record fd")).To(Equal([]string{"pass"}))
	})

	It("keeps what was recorded when the run is killed at its time limit", func() {
		run, took := runWithin(map[string]string{"a.test.ts": header + `
test("finishes", () => {});
test("never finishes", async () => { await new Promise(() => setInterval(() => {}, 1000)); });
`}, 3*time.Second)

		Expect(took).To(BeNumerically("<", bound))
		Expect(statusOf(run, "a.test.ts::finishes")).To(Equal([]string{"pass"}))
		Expect(statusOf(run, "a.test.ts::never finishes")).NotTo(ContainElement("pass"))
	})
})
