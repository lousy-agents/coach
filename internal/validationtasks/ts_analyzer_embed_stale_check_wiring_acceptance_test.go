package validationtasks

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ts-analyzer-embed-stale-check wiring", func() {
	var toml string

	BeforeEach(func() {
		raw, err := os.ReadFile(filepath.Join("..", "..", "mise.toml"))
		Expect(err).NotTo(HaveOccurred())
		toml = string(raw)
	})

	// js-verify is the only CI job that runs js-ci -- that's the external
	// fact that makes staying in js-ci's depends list (not just existing)
	// the load-bearing half of this guarantee.
	When("the generated analyzer embed changes", func() {
		It("has a task that detects drift", func() {
			Expect(taskBody(toml, "ts-analyzer-embed-stale-check")).NotTo(BeEmpty(),
				"without this, nothing in the repository notices a stale embedded analyzer")
		})

		It("is reachable from the JS job, which is the one with Node", func() {
			Expect(taskBody(toml, "js-ci")).To(ContainSubstring("ts-analyzer-embed-stale-check"),
				"a task no job depends on is a task that never runs")
		})
	})
})
