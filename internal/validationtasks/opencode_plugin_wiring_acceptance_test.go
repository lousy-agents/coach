package validationtasks

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("OpenCode plugin validation", func() {
	var toml string

	BeforeEach(func() {
		raw, err := os.ReadFile(filepath.Join("..", "..", "mise.toml"))
		Expect(err).NotTo(HaveOccurred())
		toml = string(raw)
	})

	// The plugin mirrors .claude/agents and .claude/commands into OpenCode, so
	// it is where a Claude-only mechanism becomes a broken instruction in
	// another harness. Its tests existed but no task ran them, and they sat red
	// for an agent added in #245 -- an exact-list assertion nobody saw fail.
	When("the OpenCode plugin or the files it mirrors change", func() {
		It("has a task that runs the plugin's tests", func() {
			Expect(taskBody(toml, "opencode-plugin-test")).NotTo(BeEmpty(),
				"these tests are how a harness-parity break is caught before a human hits it")
		})

		It("is reachable from the JS job, which is the one with Node", func() {
			Expect(taskBody(toml, "js-ci")).To(ContainSubstring("opencode-plugin-test"),
				"an unwired test suite reports nothing, however green it is")
		})
	})
})
