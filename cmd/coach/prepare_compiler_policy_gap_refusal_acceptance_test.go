package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
)

var _ = Describe("coach codesignal --prepare-compiler refusing behind a committed policy that is gone from disk", func() {
	const policy = "pol.json"
	var (
		repo   string
		stderr []byte
	)

	BeforeEach(func() {
		repo = commitScannableGoRepo()
		commitFile(repo, policy, singleRootPolicyJSON)
		Expect(os.Remove(filepath.Join(repo, policy))).To(Succeed())

		stdinFile, err := os.Open(os.DevNull)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(stdinFile.Close)
		stdoutFile, err := os.CreateTemp(GinkgoT().TempDir(), "stdout")
		Expect(err).NotTo(HaveOccurred())
		stderrFile, err := os.CreateTemp(GinkgoT().TempDir(), "stderr")
		Expect(err).NotTo(HaveOccurred())

		exitCode := prepareCompilerMiseTypeScript(repo, stdinFile, stdoutFile, stderrFile, policyref.Named(policy))

		Expect(exitCode).To(Equal(2))
		stderr, err = os.ReadFile(stderrFile.Name())
		Expect(err).NotTo(HaveOccurred())
	})

	It("names restore_policy and the restore command for the revision the setup analyzed", func() {
		revision, err := gitrepo.ResolveBaselineRevision(repo)
		Expect(err).NotTo(HaveOccurred())

		Expect(string(stderr)).To(ContainSubstring("(policy_missing)"))
		Expect(string(stderr)).To(ContainSubstring("restore_policy"))
		Expect(string(stderr)).To(ContainSubstring("git restore --source=" + revision + " --staged --worktree -- ':(top,literal)pol.json'"))
		Expect(string(stderr)).NotTo(ContainSubstring("author_policy"))
	})

	It("prints a command that restores the policy when it is pasted into a shell", func() {
		command := restoreCommandFrom(stderr)
		Expect(command).NotTo(BeEmpty(), "the refusal offers no command:\n%s", stderr)

		output, exitCode := runInShell(repo, command)

		Expect(exitCode).To(Equal(0), "command=%s output=%s", command, output)
		Expect(filepath.Join(repo, policy)).To(BeARegularFile())
	})
})
