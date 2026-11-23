package gitrepo

import (
	"path/filepath"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// gitBefore218 answers as git 2.17.1 does for an ordinary clone: it has no `git config --type`, which came in 2.18,
// and exits 129 on it, as git does for an option it does not know.
func gitBefore218(_ string, args ...string) ([]byte, error) {
	switch {
	case args[0] == "version":
		return []byte("git version 2.17.1\n"), nil
	case args[0] == "config" && slices.Contains(args, "--type=bool"):
		return nil, exitingWith(129)
	case args[0] == "config":
		return nil, exitStatusOne
	case args[0] == "rev-parse":
		return []byte(firstMissing + "\n"), nil
	case args[0] == "rev-list":
		return nil, nil
	}
	return nil, exitStatus128
}

var _ = Describe("whether a repository is a partial clone, asked of a git before 2.44 (#737)", func() {
	It("#737: reads an ordinary clone on git 2.17, which is inside Coach's documented minimum of 2.16", func() {
		Expect(RefuseMissingObjects(gitBefore218, "repo", "")).To(Succeed())
	})

	It("#737: counts a remote that has a partial clone filter and no promisor setting as a partial clone, as git does, so the status of a --worktree scan does not compare blobs it may lack", func() {
		t := GinkgoT()
		source := gitfixture.Init(t)
		gitfixture.CommitFile(t, source, "a.go", "package a\n")
		clone := filepath.Join(t.TempDir(), "part")
		gitfixture.Git(t, source, "config", "uploadpack.allowFilter", "true")
		gitfixture.Git(t, t.TempDir(), "clone", "--quiet", "--filter=blob:none", "file://"+source, clone)
		gitfixture.Git(t, clone, "config", "--unset", "remote.origin.promisor")
		Expect(gitfixture.Git(t, clone, "config", "--get", "remote.origin.partialclonefilter")).To(ContainSubstring("blob:none"))

		Expect(mayBeAPartialClone(RunBytes, clone, false)).To(BeTrue())
	})
})
