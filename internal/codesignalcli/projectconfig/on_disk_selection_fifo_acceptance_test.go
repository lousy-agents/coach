//go:build unix

package projectconfig

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("the policy the working tree selects", func() {
	const legacyBytes = `{"schema_version":"1","roots":["pkg"]}`

	var repo string

	BeforeEach(func() {
		repo = gitfixture.Init(GinkgoT())
		gitfixture.CommitFile(GinkgoT(), repo, "README.md", "x")
	})

	When("Coach discovered the policy", func() {
		It("does not wait on a FIFO at the canonical path: it refuses it as a not_a_regular_file and does not fall through to project.json", func() {
			Expect(os.MkdirAll(filepath.Join(repo, ".lousy-agents"), 0o755)).To(Succeed())
			Expect(syscall.Mkfifo(filepath.Join(repo, CanonicalPath), 0o644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(repo, DefaultPath), []byte(legacyBytes), 0o644)).To(Succeed())

			selected, err := SelectOnDisk(repo, "")

			var refusal *DiskPolicyRefusal
			Expect(errors.As(err, &refusal)).To(BeTrue(), "error: %v", err)
			Expect(*refusal).To(Equal(DiskPolicyRefusal{Path: CanonicalPath, Reason: "not a regular file"}))
			Expect(selected).To(Equal(DiskSelection{}))
		})
	})
})
