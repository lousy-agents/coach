package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type policyOnDisk struct {
	arrange         func(repo string)
	wantDisclosures []string
}

var _ = Describe("coach codesignal --check-project: a valid policy file on disk is the policy, whatever the index and HEAD hold at its path", func() {
	committedWithSrcRoot := func(repo string) {
		commitFile(repo, fromDiskPolicy, srcRootPolicyJSON)
		writeWorktreeFile(repo, fromDiskPolicy, singleRootPolicyJSON)
	}

	DescribeTable("passes and discloses what differs from HEAD without changing the status",
		func(row policyOnDisk) {
			repo := typescriptRepoForPolicyFromDisk()
			row.arrange(repo)

			requireThePolicyOnDiskToPass(repo, row.wantDisclosures...)
		},
		Entry("a file that nothing commits or stages", policyOnDisk{func(repo string) {
			writeWorktreeFile(repo, fromDiskPolicy, singleRootPolicyJSON)
		}, []string{notCommittedWarning}}),
		Entry("a copy staged and never committed", policyOnDisk{func(repo string) {
			writeWorktreeFile(repo, fromDiskPolicy, singleRootPolicyJSON)
			gitIn(repo, "add", fromDiskPolicy)
		}, []string{notCommittedWarning}}),
		Entry("an entry that holds no content yet (git add --intent-to-add)", policyOnDisk{func(repo string) {
			writeWorktreeFile(repo, fromDiskPolicy, singleRootPolicyJSON)
			gitIn(repo, "add", "--intent-to-add", fromDiskPolicy)
		}, []string{notCommittedWarning}}),
		Entry("a committed policy edited on disk", policyOnDisk{committedWithSrcRoot, []string{differsWarning}}),
		Entry("a committed policy with the edit staged", policyOnDisk{func(repo string) {
			committedWithSrcRoot(repo)
			gitIn(repo, "add", fromDiskPolicy)
		}, []string{differsWarning}}),
		Entry("a committed policy with its skip-worktree bit set and the file edited", policyOnDisk{func(repo string) {
			commitFile(repo, fromDiskPolicy, srcRootPolicyJSON)
			gitIn(repo, "update-index", "--skip-worktree", fromDiskPolicy)
			writeWorktreeFile(repo, fromDiskPolicy, singleRootPolicyJSON)
		}, []string{differsWarning}}),
		Entry("a committed policy a sparse checkout leaves out, with the file edited", policyOnDisk{func(repo string) {
			commitFile(repo, fromDiskPolicy, srcRootPolicyJSON)
			gitIn(repo, "config", "core.sparseCheckout", "true")
			gitIn(repo, "update-index", "--skip-worktree", fromDiskPolicy)
			writeWorktreeFile(repo, fromDiskPolicy, singleRootPolicyJSON)
		}, []string{differsWarning}}),
		Entry("a policy with an unresolved conflict in the index", policyOnDisk{func(repo string) {
			mergeConflictOver(repo, fromDiskPolicy)
			writeWorktreeFile(repo, fromDiskPolicy, singleRootPolicyJSON)
		}, []string{differsWarning}}),
		Entry("a policy committed as a symbolic link", policyOnDisk{func(repo string) {
			commitLinkThenWriteFile(repo, fromDiskPolicy)
		}, []string{differsWarning}}),
		Entry("a policy path under which HEAD holds files", policyOnDisk{func(repo string) {
			commitDirectoryThenWriteFile(repo, fromDiskPolicy)
		}, []string{notCommittedWarning}}),
		Entry("a committed policy while another git process holds the index", policyOnDisk{func(repo string) {
			commitFile(repo, fromDiskPolicy, singleRootPolicyJSON)
			Expect(os.WriteFile(filepath.Join(repo, ".git", "index.lock"), nil, 0o644)).To(Succeed())
		}, []string{unreadableWarning}}),
	)

	It("discloses nothing for a file that is the committed copy", func() {
		repo := typescriptRepoForPolicyFromDisk()
		commitFile(repo, fromDiskPolicy, singleRootPolicyJSON)

		requireThePolicyOnDiskToPass(repo)
	})

	It("passes a policy whose blob a partial clone does not have, when the file is on disk, and discloses that HEAD's copy cannot be read", func() {
		skipUnlessGitCanRefuseLazyFetch()
		clone := commitPartialClone(func(string) string { return unreachableRemote })
		putPolicyOnDisk(clone)

		doc := expectCheckProject(clone, "--project-config", fromDiskPolicy)

		Expect(doc.Checks.Policy.State).To(Equal("pass"), "gaps: %+v", doc.Gaps)
		Expect(policyDisclosureCodes(doc)).To(ConsistOf(unreadableWarning))
	})

	It("prints the disclosure in the text with the other warnings", func() {
		repo := typescriptRepoForPolicyFromDisk()
		writeWorktreeFile(repo, fromDiskPolicy, singleRootPolicyJSON)

		text := checkProjectText(repo, "--project-config", fromDiskPolicy)

		Expect(text).To(MatchRegexp(`(?s)\nWarnings:\n.*`+notCommittedWarning), "text:\n%s", text)
		Expect(text).NotTo(ContainSubstring("policy_missing"))
	})
})

var _ = Describe("coach codesignal --check-project: the roots of a policy on disk are the roots of the file read, not of the committed copy", func() {
	const policy = fromDiskPolicy

	editBoth := func(repo string) {
		Expect(os.WriteFile(filepath.Join(repo, "src", "a.ts"), []byte("export const edited = 2\n"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repo, "lib", "b.ts"), []byte("export const edited = 2\n"), 0o644)).To(Succeed())
	}

	It("lists an uncommitted path under the roots of the file on disk, and not one under the committed roots", func() {
		repo := typescriptRepoForPolicyFromDisk()
		commitFile(repo, policy, srcRootPolicyJSON)
		writeWorktreeFile(repo, policy, libRootPolicyJSON)
		editBoth(repo)

		doc := expectCheckProject(repo, "--project-config", policy)

		Expect(doc.Checks.Policy.State).To(Equal("pass"), "gaps: %+v", doc.Gaps)
		Expect(doc.DirtyWorktree.Paths).To(ContainElement("lib/b.ts"), "the disk roots are lib")
		Expect(doc.DirtyWorktree.Paths).NotTo(ContainElement("src/a.ts"), "the committed roots are not the roots read")
		Expect(doc.DirtyWorktree.Paths).NotTo(ContainElement(policy))
	})

	It("lists an uncommitted path under the roots of a file that nothing commits", func() {
		repo := typescriptRepoForPolicyFromDisk()
		writeWorktreeFile(repo, policy, libRootPolicyJSON)
		editBoth(repo)

		doc := expectCheckProject(repo, "--project-config", policy)

		Expect(doc.Checks.Policy.State).To(Equal("pass"), "gaps: %+v", doc.Gaps)
		Expect(doc.DirtyWorktree.Paths).To(ContainElement("lib/b.ts"))
		Expect(doc.DirtyWorktree.Paths).NotTo(ContainElement("src/a.ts"))
	})
})
