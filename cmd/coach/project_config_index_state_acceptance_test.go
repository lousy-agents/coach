package main

import (
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func scanExplicitPolicy(repo, policy string) []byte {
	stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", policy, "--format=json")
	ExpectWithOffset(1, exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
	return stderr
}

var _ = Describe("coach codesignal: what the message says about a policy the index holds in an unusual state", func() {
	const nested = "cfg/p.json"
	var repo string

	BeforeEach(func() {
		repo = commitScannableGoRepo()
		commitFile(repo, nested, singleRootPolicyJSON)
	})

	When("the policy is committed, its skip-worktree bit was set by hand, and the file was removed", func() {
		It("names the skip-worktree bit and the command that clears it, and does not blame a sparse checkout", func() {
			gitIn(repo, "update-index", "--skip-worktree", nested)
			Expect(os.Remove(filepath.Join(repo, nested))).To(Succeed())

			stderr := scanExplicitPolicy(repo, nested)

			Expect(string(stderr)).To(ContainSubstring("skip-worktree"), "stderr:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring("git update-index --no-skip-worktree"), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("sparse checkout"), "no sparse checkout is in use:\n%s", stderr)
		})
	})

	When("the policy is in an unresolved merge conflict and the file was removed", func() {
		It("says there is an unresolved merge conflict and does not send the user to git diff --cached", func() {
			gitIn(repo, "checkout", "-q", "-b", "other")
			commitFile(repo, nested, `{"schema_version":"1","roots":["other"]}`+"\n")
			gitIn(repo, "checkout", "-q", "-")
			commitFile(repo, nested, `{"schema_version":"1","roots":["ours"]}`+"\n")
			merge := exec.Command("git", "merge", "other")
			merge.Dir = repo
			merge.Env = commitEnv
			Expect(merge.Run()).NotTo(Succeed(), "the merge must conflict on the policy")
			Expect(os.Remove(filepath.Join(repo, nested))).To(Succeed())

			stderr := scanExplicitPolicy(repo, nested)

			Expect(string(stderr)).To(ContainSubstring("unresolved merge conflict"), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("git diff --cached"), "an unmerged path has no staged edit to review:\n%s", stderr)
		})
	})

	When("the policy was staged but never committed, and the file was removed", func() {
		const staged = "new.json"

		BeforeEach(func() {
			Expect(os.WriteFile(filepath.Join(repo, staged), []byte(singleRootPolicyJSON), 0o644)).To(Succeed())
			gitIn(repo, "add", staged)
			Expect(os.Remove(filepath.Join(repo, staged))).To(Succeed())
		})

		It("offers no new policy, says the index holds a copy, and prints a command that brings the file back", func() {
			stderr := scanExplicitPolicy(repo, staged)

			Expect(string(stderr)).NotTo(ContainSubstring(suggestCommand), "a staged copy exists, so authoring must not be offered:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring("staged copy"), "stderr:\n%s", stderr)
			command := restoreCommandFrom(stderr)
			Expect(command).NotTo(BeEmpty(), "the message offers no command:\n%s", stderr)
			output, exitCode := runInShell(repo, command)
			Expect(exitCode).To(Equal(0), "command=%s output=%s", command, output)
			Expect(filepath.Join(repo, staged)).To(BeARegularFile())
			Expect(gitIn(repo, "status", "--porcelain", "--", staged)).To(HavePrefix("A"), "the staged add is untouched")
		})

		It("names neither author_policy nor restore_policy in --check-project", func() {
			doc := expectCheckProject(repo, "--project-config", staged)

			Expect(doc.Checks.Policy.Code).To(Equal("policy_missing"))
			Expect(nextActionKinds(doc)).NotTo(ContainElement("author_policy"))
		})
	})
})
