package agentworkflows

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The implement/review loop's validation command decides whether red-then-green
// evidence is real. `mise run ci` runs the Go suite before the TypeScript
// sidecar is built, so pkg/projectmodel's acceptance suite skips silently there
// -- an acceptance test living in that suite would produce a "red" and a "green"
// that are the same skip. Naming the command in each contract is what keeps that
// out of the loop; inheriting it from AGENTS.md leaves the choice to the agent.
var _ = Describe("per-cycle validation contract", func() {
	DescribeTable("names the sidecar-first command explicitly",
		func(agent string) {
			body, err := os.ReadFile(filepath.Join("..", "..", ".claude", "agents", agent))
			Expect(err).NotTo(HaveOccurred())
			text := string(body)

			Expect(text).To(ContainSubstring("ci-fast"),
				"%s must name the command rather than leaving it to be inferred", agent)
			Expect(strings.Contains(text, "skips silently") || strings.Contains(text, "silent")).To(BeTrue(),
				"%s must say why, or the next edit will 'simplify' it back to ci", agent)
		},
		Entry("implementer", "task-implementer.md"),
		Entry("reviewer", "task-reviewer.md"),
	)
})
