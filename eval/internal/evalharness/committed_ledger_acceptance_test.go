package evalharness_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalharness"
)

var _ = Describe("the run ledger committed to the repository", func() {
	It("loads line by line under the current schemas, so every tool that reads it can read it", func() {
		lines, err := evalharness.LoadLedger("../../ledger/runs.jsonl")

		Expect(err).NotTo(HaveOccurred())
		Expect(lines).NotTo(BeEmpty())
	})
})
