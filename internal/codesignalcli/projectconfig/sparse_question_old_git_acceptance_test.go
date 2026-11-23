package projectconfig

import (
	"os/exec"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// sparseOnGitBefore218 answers as git 2.17 does in a repository rooted at root whose sparse checkout is on:
// it has no `git config --type`, which came in 2.18, and exits 129 on it, as git does for an option it does not
// know.
func sparseOnGitBefore218(root string) gitrepo.Runner {
	return func(_ string, args ...string) ([]byte, error) {
		switch {
		case args[0] == "rev-parse":
			return []byte(root + "\n"), nil
		case args[0] == "config" && slices.Contains(args, "--type=bool"):
			return nil, exec.Command("sh", "-c", "exit 129").Run()
		case args[0] == "config" && slices.Contains(args, "core.sparseCheckout"):
			return []byte("true\n"), nil
		}
		return nil, exec.Command("sh", "-c", "exit 128").Run()
	}
}

var _ = Describe("whether sparse checkout is on, asked of git 2.17 (#741)", func() {
	It("#741: is answered for a new policy whose directory is not on disk, so the sparse note is offered", func() {
		run := sparseOnGitBefore218(GinkgoT().TempDir())

		Expect(newPolicyMayNeedSparse(run, "repo", "outside/the/checkout/policy.json")).To(BeTrue())
	})

	It("#741: is answered for a staged policy that sparse checkout skips, rather than failing the question", func() {
		run := sparseOnGitBefore218(GinkgoT().TempDir())

		enabled, err := sparseCheckoutEnabled(run, "repo", []gitrepo.IndexEntry{{SkipWorktree: true}})

		Expect(err).NotTo(HaveOccurred())
		Expect(enabled).To(BeTrue())
	})
})
