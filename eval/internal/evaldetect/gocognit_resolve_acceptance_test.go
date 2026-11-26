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

	When("the module cache does not hold the pinned version", func() {
		It("refuses without fetching anything", func() {
			emptyCache := GinkgoT().TempDir()

			_, err := evaldetect.ResolveGocognit(evalModule, []string{"GOMODCACHE=" + emptyCache})
			Expect(err).To(MatchError(evaldetect.ErrGocognitUnavailable))

			Expect(downloadedFiles(emptyCache)).To(BeEmpty(), "resolving gocognit must not download into the module cache")
		})
	})
})
