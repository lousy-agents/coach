package acceptancecienv

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("copying a checkout's .git while git holds a lock (#867)", func() {
	When("git removes index.lock after the copy has listed it", func() {
		It("copies the repository without failing on the lock, and copies no lock file", func() {
			source := filepath.Join(GinkgoT().TempDir(), ".git")
			for name, content := range map[string]string{
				"HEAD":                 "ref: refs/heads/main\n",
				"index":                "index",
				"index.lock":           "held",
				"refs/heads/main":      "0123456789abcdef0123456789abcdef01234567\n",
				"refs/heads/main.lock": "held",
			} {
				Expect(os.MkdirAll(filepath.Dir(filepath.Join(source, name)), 0o755)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(source, name), []byte(content), 0o644)).To(Succeed())
			}
			target := filepath.Join(GinkgoT().TempDir(), ".git")

			err := copyGitDir(source, target, func() {
				Expect(os.Remove(filepath.Join(source, "index.lock"))).To(Succeed())
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(filepath.Join(target, "index")).To(BeAnExistingFile())
			Expect(filepath.Join(target, "refs", "heads", "main")).To(BeAnExistingFile())
			Expect(filepath.Join(target, "index.lock")).NotTo(BeAnExistingFile())
			Expect(filepath.Join(target, "refs", "heads", "main.lock")).NotTo(BeAnExistingFile())
		})
	})
})
