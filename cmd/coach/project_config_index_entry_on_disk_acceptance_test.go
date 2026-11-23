package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A policy that nothing commits and that the index still holds an entry for: a scan reads the file when it is on disk and
// names what the index holds when it is not, and --check-project keeps the path in dirty_worktree, because nothing read
// what the index holds. A file that is on disk decides the gap by its presence (the check read it), which is true of a
// file the index knows nothing about, and false of one the index holds a staged copy or an unresolved conflict for.
var _ = Describe("coach codesignal: a policy that is not committed and that the index holds, with the file on disk and with it deleted", func() {
	const policy = "cfg/p.json"

	stagedNeverCommitted := func(repo string) {
		writeWorktreeFile(repo, policy, singleRootPolicyJSON)
		gitIn(repo, "add", policy)
	}
	deletedByUs := func(repo string) { modifyDeleteConflictOver(repo, policy, true) }
	intentToAdd := func(repo string) {
		writeWorktreeFile(repo, policy, singleRootPolicyJSON)
		gitIn(repo, "add", "--intent-to-add", policy)
	}

	// arranged is a repository the index holds the policy in the state arrange leaves it, with the file where the index
	// left it or removed from disk.
	arranged := func(repo string, arrange func(repo string), onDisk bool) string {
		arrange(repo)
		if !onDisk {
			removeTree(repo, "cfg")
		}
		return repo
	}
	typescriptRepo := func() string {
		repo := newTempGitRepo()
		commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
		commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
		commitFile(repo, "src/a.ts", "export const a = 1\n")
		return repo
	}

	DescribeTable("the scan says what the index holds, and says no advice that git refuses",
		func(arrange func(repo string), onDisk bool, want string, unwanted string) {
			repo := arranged(commitScannableGoRepo(), arrange, onDisk)

			stderr := scanExplicitPolicy(repo, policy)

			Expect(string(stderr)).To(ContainSubstring(want), "stderr:\n%s", stderr)
			if unwanted != "" {
				Expect(string(stderr)).NotTo(ContainSubstring(unwanted), "stderr:\n%s", stderr)
			}
		},
		Entry("a staged copy, the file deleted", stagedNeverCommitted, false, "staged copy", "merge conflict"),
		Entry("a conflict one side deleted, the file deleted", deletedByUs, false, "merge conflict", "commit the file"),
	)

	DescribeTable("a scan reads the file that is on disk, whatever the index holds for it, and discloses it as not committed at HEAD",
		func(arrange func(repo string)) {
			repo := arranged(commitScannableGoRepo(), arrange, true)
			onDisk, err := os.ReadFile(filepath.Join(repo, policy))
			Expect(err).NotTo(HaveOccurred())

			stdout := scanExplicitPolicyAccepted(repo, policy)

			Expect(scopePolicyScopeOf([]byte(stdout))).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(policy, string(onDisk), diskCauseNotCommitted))))
		},
		Entry("a staged copy, the file on disk", stagedNeverCommitted),
		Entry("a conflict one side deleted, the file on disk", deletedByUs),
		Entry("an entry that holds no content (git add --intent-to-add), the file on disk", intentToAdd),
	)

	DescribeTable("--check-project passes a policy file that is on disk whatever the index holds, and leaves its path out of dirty_worktree",
		func(arrange func(repo string)) {
			repo := arranged(typescriptRepo(), arrange, true)

			doc := expectCheckProject(repo, "--project-config", policy)

			Expect(doc.Checks.Policy.State).To(Equal("pass"), "gaps: %+v", doc.Gaps)
			Expect(gapCodes(doc)).NotTo(ContainElement("policy_missing"))
			Expect(doc.DirtyWorktree.Paths).NotTo(ContainElement(policy), "the check read the file")
		},
		Entry("a staged copy, the file on disk", stagedNeverCommitted),
		Entry("a conflict one side deleted, the file on disk", deletedByUs),
		Entry("an entry that holds no content (git add --intent-to-add), the file on disk", intentToAdd),
	)

	DescribeTable("--check-project names no cause the index decided for a policy file that is deleted, and no restore from HEAD",
		func(arrange func(repo string)) {
			repo := arranged(typescriptRepo(), arrange, false)

			doc := expectCheckProject(repo, "--project-config", policy)

			Expect(causeOfGap(doc, "policy_missing")).To(Equal("undetermined"))
			Expect(nextActionKinds(doc)).NotTo(ContainElement("restore_policy"))
			Expect(nextActionKinds(doc)).NotTo(ContainElement("author_policy"))
			Expect(doc.DirtyWorktree.Paths).NotTo(ContainElement(policy), "the check looked for the file on disk")
		},
		Entry("a staged copy, the file deleted", stagedNeverCommitted),
		Entry("a conflict one side deleted, the file deleted", deletedByUs),
	)
})

var _ = Describe("the CLI contract: what the index decides about a policy that is on disk and not committed", func() {
	It("says an unresolved conflict is merge_conflict and not not_committed, and that the path stays in dirty_worktree with the file on disk or gone", func() {
		text := documentText("docs/cli-codesignal.md")

		Expect(text).To(ContainSubstring("is `policy_missing` with the cause `merge_conflict` and not `not_committed`"))
		Expect(text).To(ContainSubstring("A staged copy of it keeps `not_committed`, because committing it is the way out."))
		Expect(text).To(ContainSubstring("In both the path stays in `dirty_worktree`, with the file on disk or gone."))
	})
})
