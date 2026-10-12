package baseline_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/coachapi/baseline"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

var _ = Describe("repo_baseline_scan job handler", func() {
	When("the local smoke fixture contains a symlink that points outside the root", func() {
		It("skips the symlink in ListFiles and rejects it on ReadFile", func() {
			expectFixtureSymlinkSkippedAndRejected()
		})
	})

	When("the local smoke fixture has supported sources under a top-level dot directory", func() {
		It("lists them like GitHub Contents (does not drop paths starting with '.')", func() {
			expectDotDirectorySourcesListed()
		})
	})

	When("the local fixture's eligible file count equals MaxFiles exactly", func() {
		It("admits all of them, and one file over MaxFiles trips the budget", func() {
			expectMaxFilesBudgetBoundary()
		})
	})

	When("the local fixture's eligible-file total size equals MaxTotalBytes exactly", func() {
		It("admits it, and one byte over trips the budget", func() {
			root := GinkgoT().TempDir()
			content := []byte("package f\n")
			Expect(os.WriteFile(filepath.Join(root, "f.go"), content, 0o644)).To(Succeed())

			src := &baseline.LocalFixtureTreeSource{Root: root}
			entries, err := src.ListFiles(context.Background(), "o", "r", "", baseline.ListOptions{MaxTotalBytes: int64(len(content))})
			Expect(err).NotTo(HaveOccurred(), "a fixture whose total size equals MaxTotalBytes exactly must be admitted, not rejected")
			Expect(entries).To(HaveLen(1))

			_, err = src.ListFiles(context.Background(), "o", "r", "", baseline.ListOptions{MaxTotalBytes: int64(len(content)) - 1})
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, githubingest.ErrTooLarge)).To(BeTrue(), "one byte over MaxTotalBytes must trip the budget; got %v", err)
		})
	})
})
