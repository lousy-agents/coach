package evaldetect_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evaldetect"
)

var _ = Describe("resolving the pinned gocognit", func() {
	evalModule := filepath.Join("..", "..")

	When("the module cache holds the pinned version", func() {
		It("resolves a binary that passes the version check", func() {
			gocognit, err := evaldetect.ResolveGocognit(evalModule, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(gocognit.Version).To(Equal(pinnedGocognit))
			Expect(gocognit.CheckVersion()).To(Succeed())
		})
	})

	When("the module cache does not hold the pinned version", func() {
		It("refuses without fetching anything", func() {
			emptyCache := GinkgoT().TempDir()

			_, err := evaldetect.ResolveGocognit(evalModule, []string{"GOMODCACHE=" + emptyCache})
			Expect(err).To(MatchError(evaldetect.ErrGocognitUnavailable))

			entries, readErr := os.ReadDir(emptyCache)
			Expect(readErr).NotTo(HaveOccurred())
			Expect(entries).To(BeEmpty(), "resolving gocognit must not download into the module cache")
		})
	})
})
