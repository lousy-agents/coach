package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func gitInWithIdentity(repo string, args ...string) {
	command := exec.Command("git", args...)
	command.Dir = repo
	command.Env = commitEnv
	_, _ = command.CombinedOutput()
}

var _ = Describe("coach codesignal: the message for a policy that nothing commits and the index still holds", func() {
	const policy = "cfg/p.json"
	var repo string

	BeforeEach(func() {
		repo = commitScannableGoRepo()
	})

	When("the index holds it only as an unresolved merge conflict and the file was removed", func() {
		It("names the conflict and prints no restore command, because git restore refuses an unmerged path", func() {
			commitFile(repo, policy, singleRootPolicyJSON)
			trunk := strings.TrimSpace(gitIn(repo, "rev-parse", "--abbrev-ref", "HEAD"))
			gitIn(repo, "checkout", "-q", "-b", "side")
			commitFile(repo, policy, singleRootPolicyJSON+"\n")
			gitIn(repo, "checkout", "-q", trunk)
			gitIn(repo, "rm", "-q", policy)
			gitInWithIdentity(repo, "commit", "-q", "-m", "drop the policy")
			gitInWithIdentity(repo, "merge", "side")
			Expect(gitIn(repo, "ls-files", "--stage", "--", policy)).To(ContainSubstring(" 1\t"), "the merge must leave the policy unmerged")
			Expect(os.RemoveAll(filepath.Join(repo, "cfg"))).To(Succeed())

			stderr := scanExplicitPolicy(repo, policy)

			Expect(string(stderr)).To(ContainSubstring("merge conflict"), "stderr:\n%s", stderr)
			Expect(restoreCommandFrom(stderr)).To(BeEmpty(), "git restore fails on an unmerged path, so no command may be printed:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("staged copy"), "stderr:\n%s", stderr)
		})
	})

	When("it was staged as a symbolic link, never committed, and the link was removed", func() {
		It("prints no restore command, because that would recreate a link the scan refuses", func() {
			Expect(os.MkdirAll(filepath.Join(repo, "cfg"), 0o755)).To(Succeed())
			Expect(os.Symlink("../real.json", filepath.Join(repo, policy))).To(Succeed())
			gitIn(repo, "add", policy)
			Expect(os.Remove(filepath.Join(repo, policy))).To(Succeed())

			stderr := scanExplicitPolicy(repo, policy)

			Expect(string(stderr)).To(ContainSubstring("symbolic link"), "stderr:\n%s", stderr)
			Expect(restoreCommandFrom(stderr)).To(BeEmpty(), "a policy cannot be a symbolic link, so no restore may be offered:\n%s", stderr)
		})
	})
})
