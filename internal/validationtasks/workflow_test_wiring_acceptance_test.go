package validationtasks

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("workflow script validation", func() {
	var toml string

	BeforeEach(func() {
		raw, err := os.ReadFile(filepath.Join("..", "..", "mise.toml"))
		Expect(err).NotTo(HaveOccurred())
		toml = string(raw)
	})

	// A workflow script is executed by the Workflow tool, not compiled or
	// imported by anything else here, so a syntax error or a renamed binding
	// surfaces only when a human runs the command. The Go suite cannot catch
	// it -- the `verify` job has no Node -- so the check has to live on a task
	// that runs where Node exists, and it has to stay reachable from CI.
	When("a workflow script changes", func() {
		It("has a task that exercises it", func() {
			Expect(taskBody(toml, "workflow-test")).NotTo(BeEmpty(),
				"without this, nothing in the repository notices a broken workflow script")
		})

		It("is reachable from the JS job, which is the one with Node", func() {
			Expect(taskBody(toml, "js-ci")).To(ContainSubstring("workflow-test"),
				"a task no job depends on is a task that never runs")
		})

		// ci-fast is the loop an implementer actually runs between edits. These
		// suites cover the agent-tooling files most likely to be edited in that
		// loop and finish in well under a second, so leaving them out means the
		// fastest feedback available is the ~400s gate.
		It("runs in the per-cycle loop, not only in the full gate", func() {
			body := taskBody(toml, "ci-fast")
			Expect(body).To(ContainSubstring("workflow-test"))
			Expect(body).To(ContainSubstring("opencode-plugin-test"))
		})
	})
})
