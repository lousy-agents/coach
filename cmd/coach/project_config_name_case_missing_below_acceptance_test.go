package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A parent typed in the wrong case is refused even when a deeper directory is missing too. Authoring the
// policy at that path would write it where the file system resolves the parent, in a directory the scan then
// refuses, and git records it under the other spelling: following the advice must not be a trap.
var _ = Describe("coach codesignal: a missing policy under a parent typed in the wrong case is refused for the parent", func() {
	BeforeEach(func() {
		skipUnlessFilesystemResolvesCase()
	})

	DescribeTable("when a directory below the parent does not exist either",
		func(typed, spelledOnDisk string) {
			repo := commitScannableGoRepo()
			commitFile(repo, spelledOnDisk+"/p.json", singleRootPolicyJSON)

			stderr := scanExplicitPolicy(repo, typed+"/new/zz.json")
			doc := expectCheckProject(repo, "--project-config", typed+"/new/zz.json")

			Expect(string(stderr)).To(ContainSubstring(`is stored as "`+spelledOnDisk+`" on disk, not "`+typed+`"`), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring(suggestCommand), "authoring would write it where the other spelling resolves:\n%s", stderr)
			causes := []string{}
			for _, gap := range doc.Gaps {
				causes = append(causes, gap.Cause)
			}
			Expect(causes).To(ContainElement("name_case_differs"), "gaps: %+v", doc.Gaps)
			_, authors := nextActionOfKind(doc, "author_policy")
			Expect(authors).To(BeFalse(), "next actions: %+v", doc.NextActions)
		},
		Entry("a configuration directory", "Cfg", "cfg"),
		Entry("a source directory", "SRC", "src"),
	)

	It("still calls the policy missing and offers to author it when every directory that exists is spelled as typed", func() {
		repo := commitScannableGoRepo()
		commitFile(repo, "cfg/p.json", singleRootPolicyJSON)

		doc := expectCheckProject(repo, "--project-config", "cfg/new/zz.json")

		Expect(doc.Checks.Policy.Code).To(Equal("policy_missing"))
		_, authors := nextActionOfKind(doc, "author_policy")
		Expect(authors).To(BeTrue(), "next actions: %+v", doc.NextActions)
	})
})
