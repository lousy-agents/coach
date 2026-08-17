package claudehooks

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// AGENTS.md sends a human to the hook trace to find out what a run actually
// did -- it is the answer to "was the loop bounded", which nothing else in the
// system can answer. That makes the file's contents load-bearing, and a test
// suite that appends to it is writing fiction into the evidence.
//
// Observed: a proving run read 230 rows from this file and took them for live
// hook activity. They were synthetic rows this package's own tests had written
// during a routine `mise run ci-fast`, under session IDs no session ever had.
// The trace is only consulted when tracing is switched on, which is exactly
// when the pollution happens -- the failure is invisible until it misleads.
var _ = Describe("hook trace isolation", func() {
	// Enabling the marker is what a developer does to diagnose a run. The
	// suite has to stay clean in that state, not only in the default one.
	enableMarker := func() {
		GinkgoHelper()
		marker := filepath.Join("..", "..", ".claude", "hooks", ".trace-enabled")
		if _, err := os.Stat(marker); err == nil {
			return // already enabled by the developer; leave it exactly as found
		}
		Expect(os.WriteFile(marker, []byte("test\n"), 0o600)).To(Succeed())
		DeferCleanup(func() { Expect(os.Remove(marker)).To(Succeed()) })
	}

	repoTrace := func() string {
		GinkgoHelper()
		p, err := filepath.Abs(filepath.Join("..", "..", ".coach-hook-trace.tsv"))
		Expect(err).NotTo(HaveOccurred())
		return p
	}

	When("tracing is enabled and a test runs a hook without pinning the trace path", func() {
		It("writes to the suite's own trace, not the repository's", func() {
			enableMarker()
			before, readErr := os.ReadFile(repoTrace())
			existedBefore := readErr == nil
			DeferCleanup(func() {
				// Restore whichever state the repository was in, so a failing
				// run does not leave behind the very artefact it is about.
				if existedBefore {
					Expect(os.WriteFile(repoTrace(), before, 0o600)).To(Succeed())
					return
				}
				if err := os.Remove(repoTrace()); err != nil {
					Expect(os.IsNotExist(err)).To(BeTrue())
				}
			})

			path, err := filepath.Abs(filepath.Join("..", "..", ".claude", "hooks", "verify-review-verdict.sh"))
			Expect(err).NotTo(HaveOccurred())
			body, err := json.Marshal(map[string]any{"last_assistant_message": "PASS"})
			Expect(err).NotTo(HaveOccurred())

			// Deliberately inherits the environment as-is. Any test that forgets
			// to pin COACH_HOOK_TRACE looks exactly like this, so this is the
			// case the suite-wide default has to cover.
			cmd := exec.Command("bash", path)
			cmd.Stdin = bytes.NewReader(body)
			cmd.Env = os.Environ()
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			Expect(cmd.Run()).To(Succeed(), "stderr: %s", stderr.String())

			after, err := os.ReadFile(repoTrace())
			if !existedBefore {
				Expect(err).To(HaveOccurred(),
					"the suite created the repository's trace file; a human reading it would see rows no session produced")
				return
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(after).To(Equal(before),
				"the suite appended to the repository's trace file; those rows are indistinguishable from live ones")
		})
	})
})
