package evaldetect_test

import (
	"io/fs"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evaldetect"
)

// downloadedFiles lists every file in a module cache except the lock files
// the go command leaves even when module lookup is disabled.
func downloadedFiles(cache string) []string {
	var files []string
	Expect(filepath.WalkDir(cache, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && !strings.HasSuffix(path, ".lock") {
			files = append(files, path)
		}
		return err
	})).To(Succeed())
	return files
}

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

	DescribeTable("a caller's environment cannot turn the build into a fetch or a failure",
		func(env string) {
			gocognit, err := evaldetect.ResolveGocognit(evalModule, []string{env})
			Expect(err).NotTo(HaveOccurred())
			Expect(gocognit.CheckVersion()).To(Succeed())
		},
		Entry("a toolchain the host does not have", "GOTOOLCHAIN=go1.26.0"),
		Entry("vendor mode with no vendor directory", "GOFLAGS=-mod=vendor"),
	)

	When("the caller names a module proxy and the cache is empty", func() {
		It("still refuses without fetching anything", func() {
			emptyCache := GinkgoT().TempDir()

			_, err := evaldetect.ResolveGocognit(evalModule, []string{"GOMODCACHE=" + emptyCache, "GOPROXY=https://proxy.golang.org,direct"})
			Expect(err).To(MatchError(evaldetect.ErrGocognitUnavailable))
			Expect(downloadedFiles(emptyCache)).To(BeEmpty(), "a caller's GOPROXY must not reopen module lookup")
		})
	})

	When("the module cache does not hold the pinned version", func() {
		It("refuses without fetching anything", func() {
			emptyCache := GinkgoT().TempDir()

			_, err := evaldetect.ResolveGocognit(evalModule, []string{"GOMODCACHE=" + emptyCache})
			Expect(err).To(MatchError(evaldetect.ErrGocognitUnavailable))

			Expect(downloadedFiles(emptyCache)).To(BeEmpty(), "resolving gocognit must not download into the module cache")
		})
	})
})
