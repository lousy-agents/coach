package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A checkout whose line endings git converted is the committed copy wherever the scan starts, because the path the policy
// is named by is relative to the repository root and the copy is compared there.
var _ = Describe("coach codesignal (Go): a converted checkout of a committed invalid policy, scanned from a subdirectory", func() {
	It("offers the replacement from the policy's own directory, as it does from the root", func() {
		const policy = "cfg/p.json"
		repo := commitGoRepoWithPolicy(policy, invalidPolicy)
		gitIn(repo, "config", "core.autocrlf", "true")
		Expect(os.Remove(filepath.Join(repo, policy))).To(Succeed())
		gitIn(repo, "checkout", "--", policy)
		Expect(gitIn(repo, "status", "--porcelain")).To(BeEmpty(), "git itself calls the checkout clean")

		stdout, stderr, exitCode := runCoachBinary(commandPath, filepath.Join(repo, "cfg"), nil, "codesignal", "--baseline", "--format=json", "--project-config", policy)

		Expect(exitCode).To(Equal(2), "stdout: %s\nstderr: %s", stdout, stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("differs from the committed"), "nothing was edited:\n%s", stderr)
		Expect(lastLine(stderr)).To(ContainSubstring("replace the policy with it, commit, and rerun"), "stderr:\n%s", stderr)
	})
})
