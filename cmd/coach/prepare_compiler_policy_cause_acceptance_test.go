package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
)

// A scan prints why the policy it was told to use cannot be used. The
// --prepare-compiler refusal for the same policy has to give the same reason
// when neither authoring nor restoring can fix it, because its own line can only
// say that a new policy would not help.
var _ = Describe("coach codesignal --prepare-compiler refusing behind a policy that neither authoring nor restoring fixes", func() {
	const policy = "pol.json"

	DescribeTable("says why, in the words the scan uses, before it says what to do",
		func(arrange func(repo string), wantCause string) {
			repo := commitScannableGoRepo()
			arrange(repo)
			stdinFile, err := os.Open(os.DevNull)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(stdinFile.Close)
			stdoutFile, err := os.CreateTemp(GinkgoT().TempDir(), "stdout")
			Expect(err).NotTo(HaveOccurred())
			stderrFile, err := os.CreateTemp(GinkgoT().TempDir(), "stderr")
			Expect(err).NotTo(HaveOccurred())

			exitCode := prepareCompilerMiseTypeScript(repo, stdinFile, stdoutFile, stderrFile, policyref.Named(policy))

			Expect(exitCode).To(Equal(2))
			stderr, err := os.ReadFile(stderrFile.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(string(stderr)).To(ContainSubstring(wantCause), "stderr:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring("a reviewed, committed policy is required before compiler setup"), "stderr:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring("a new policy cannot fix it"), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("move it aside"), "a policy with work in it is not moved aside:\n%s", stderr)
		},
		Entry("the policy exists on disk but is not committed", func(repo string) {
			writeWorktreeFile(repo, policy, singleRootPolicyJSON)
		}, "commit the file so it is readable at the analyzed revision"),
		Entry("the policy is committed, its skip-worktree bit was set by hand, and the file was removed", func(repo string) {
			commitFile(repo, policy, singleRootPolicyJSON)
			gitIn(repo, "update-index", "--skip-worktree", policy)
			Expect(os.Remove(filepath.Join(repo, policy))).To(Succeed())
		}, "skip-worktree"),
		Entry("the policy is committed with a staged edit and the file was removed", func(repo string) {
			commitFile(repo, policy, singleRootPolicyJSON)
			writeWorktreeFile(repo, policy, `{"schema_version":"1","roots":["other"]}`+"\n")
			gitIn(repo, "add", policy)
			Expect(os.Remove(filepath.Join(repo, policy))).To(Succeed())
		}, "staged edit"),
		// The refusal for a committed document that is not valid says to move it aside and write a new one, but only where a new policy
		// fixes it. Where the file holds work, the remedy is none, and the refusal must say a new policy would not help.
		Entry("the committed document is not valid and a fix is on disk, not committed", func(repo string) {
			commitFile(repo, policy, invalidPolicy)
			writeWorktreeFile(repo, policy, validPolicyWithLayers)
		}, "differs from the committed"),
		Entry("the committed document is not valid and a fix is staged", func(repo string) {
			commitFile(repo, policy, invalidPolicy)
			writeWorktreeFile(repo, policy, validPolicyWithLayers)
			gitIn(repo, "add", policy)
		}, "staged edit"),
	)
})
