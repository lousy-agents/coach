package main

import (
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type policyGap struct {
	arrange func(repo string)
	code    string
}

func requireNoRestoreFromHeadAndNoAuthoring(doc readinessResultDoc, text string) {
	GinkgoHelper()
	Expect(nextActionKinds(doc)).NotTo(ContainElement("restore_policy"), "next actions: %+v", doc.NextActions)
	Expect(nextActionKinds(doc)).NotTo(ContainElement("author_policy"), "next actions: %+v", doc.NextActions)
	for _, detail := range nextActionDetails(doc) {
		Expect(detail).NotTo(ContainSubstring("--source="), "the restore from HEAD would discard what the index holds")
	}
	Expect(text).NotTo(ContainSubstring("--source="), "text:\n%s", text)
}

var _ = Describe("coach codesignal --check-project: a policy file that is absent from disk while the index holds something for it", func() {
	DescribeTable("is a gap with the cause undetermined, and never offers the restore from HEAD that would overwrite what the index holds",
		func(row policyGap) {
			repo := typescriptRepoForPolicyFromDisk()
			row.arrange(repo)

			doc := expectCheckProject(repo, "--project-config", fromDiskPolicy)
			text := checkProjectText(repo, "--project-config", fromDiskPolicy)

			Expect(doc.Checks.Policy.Code).To(Equal(row.code))
			Expect(causeOfGap(doc, row.code)).To(Equal("undetermined"), "gaps: %+v", doc.Gaps)
			Expect(text).To(ContainSubstring("  " + row.code + " cause=undetermined\n"))
			requireNoRestoreFromHeadAndNoAuthoring(doc, text)
		},
		Entry("a committed policy with an edit staged, the file deleted", policyGap{func(repo string) {
			commitFile(repo, fromDiskPolicy, singleRootPolicyJSON)
			writeWorktreeFile(repo, fromDiskPolicy, srcRootPolicyJSON)
			gitIn(repo, "add", fromDiskPolicy)
			removeTree(repo, "cfg")
		}, "policy_missing"}),
		Entry("a policy committed as a symbolic link, the link removed", policyGap{func(repo string) {
			symlinkAt(repo, fromDiskPolicy)
			gitIn(repo, "add", fromDiskPolicy)
			gitInWithIdentity(repo, "commit", "-q", "-m", "a policy as a link")
			removeTree(repo, "cfg")
		}, "policy_missing"}),
	)
})

var _ = Describe("coach codesignal --check-project: a regular file on disk whose content is not a usable policy", func() {
	oversized := strings.Repeat("x", (1<<20)+1)

	DescribeTable("is policy_invalid with the cause undetermined and no way out the check can name, whatever HEAD holds",
		func(arrange func(repo string)) {
			repo := typescriptRepoForPolicyFromDisk()
			arrange(repo)

			doc := expectCheckProject(repo, "--project-config", fromDiskPolicy)
			text := checkProjectText(repo, "--project-config", fromDiskPolicy)

			Expect(doc.Checks.Policy.Code).To(Equal("policy_invalid"))
			Expect(causeOfGap(doc, "policy_invalid")).To(Equal("undetermined"), "gaps: %+v", doc.Gaps)
			Expect(text).To(ContainSubstring("  policy_invalid cause=undetermined\n"))
			Expect(doc.Status).To(BeElementOf("needs_policy", "needs_prerequisite"), "a policy_invalid gap ranks behind a missing prerequisite, such as node on this host")
			Expect(doc.DirtyWorktree.Paths).NotTo(ContainElement(fromDiskPolicy), "the check read the file")
			requireNoRestoreFromHeadAndNoAuthoring(doc, text)
		},
		Entry("invalid content, nothing committed", func(repo string) {
			writeWorktreeFile(repo, fromDiskPolicy, invalidPolicyText)
		}),
		Entry("invalid content over a valid committed copy", func(repo string) {
			commitFile(repo, fromDiskPolicy, singleRootPolicyJSON)
			writeWorktreeFile(repo, fromDiskPolicy, invalidPolicyText)
		}),
		Entry("invalid content that is also the committed copy", func(repo string) {
			commitFile(repo, fromDiskPolicy, invalidPolicyText)
		}),
		Entry("a file over the size budget over a valid committed copy", func(repo string) {
			commitFile(repo, fromDiskPolicy, singleRootPolicyJSON)
			writeWorktreeFile(repo, fromDiskPolicy, oversized)
		}),
		Entry("conflict markers over a valid committed copy", func(repo string) {
			commitFile(repo, fromDiskPolicy, singleRootPolicyJSON)
			writeWorktreeFile(repo, fromDiskPolicy, conflictedMarkers)
		}),
	)
})

var _ = Describe("coach codesignal --check-project: the default policy committed at HEAD and deleted on disk", func() {
	It("is policy_missing with the restore from HEAD when no project.json on disk stands in for it", func() {
		repo := typescriptRepoForPolicyFromDisk()
		commitFile(repo, coachConfigPath, singleRootPolicyJSON)
		Expect(os.Remove(repo + "/" + coachConfigPath)).To(Succeed())

		doc := expectCheckProject(repo)

		Expect(doc.Checks.Policy.Code).To(Equal("policy_missing"), "gaps: %+v", doc.Gaps)
		Expect(causeOfGap(doc, "policy_missing")).To(BeEmpty())
		Expect(nextActionKinds(doc)).To(ContainElement("restore_policy"))
		Expect(nextActionKinds(doc)).NotTo(ContainElement("author_policy"))
	})

	It("passes through a valid project.json on disk and warns that project.json is deprecated", func() {
		repo := typescriptRepoForPolicyFromDisk()
		commitFile(repo, coachConfigPath, singleRootPolicyJSON)
		commitFile(repo, legacyConfigPath, singleRootPolicyJSON)
		Expect(os.Remove(repo + "/" + coachConfigPath)).To(Succeed())

		doc := expectCheckProject(repo)

		Expect(doc.Checks.Policy.State).To(Equal("pass"), "gaps: %+v", doc.Gaps)
		Expect(warningCodes(doc)).To(ContainElement("project_config_deprecated"))
		Expect(gapCodes(doc)).NotTo(ContainElement(HavePrefix("policy_")))
		Expect(nextActionKinds(doc)).NotTo(ContainElement("restore_policy"))
	})
})
