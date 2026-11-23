package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --check-project: a policy file on disk that is not valid names no author_policy", func() {
	var repo string

	BeforeEach(func() {
		repo = newTempGitRepo()
		commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
		commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
	})

	When("the committed canonical policy is not valid", func() {
		It("reports policy_invalid with the cause undetermined and offers neither authoring nor a restore", func() {
			commitFile(repo, coachConfigPath, invalidPolicyText)

			doc := expectCheckProject(repo)

			Expect(doc.Checks.Policy.Code).To(Equal("policy_invalid"))
			Expect(causeOfGap(doc, "policy_invalid")).To(Equal("undetermined"), "gaps: %+v", doc.Gaps)
			Expect(nextActionKinds(doc)).NotTo(ContainElement("author_policy"), "authoring refuses a path that exists")
			Expect(nextActionKinds(doc)).NotTo(ContainElement("restore_policy"))
		})
	})

	When("an explicit --project-config is a regular file on disk and is the committed copy, which is not valid", func() {
		It("reports policy_invalid with the cause undetermined and offers neither authoring nor a restore", func() {
			commitFile(repo, "cfg/p.json", invalidPolicyText)

			doc := expectCheckProject(repo, "--project-config", "cfg/p.json")

			Expect(doc.Checks.Policy.Code).To(Equal("policy_invalid"))
			Expect(causeOfGap(doc, "policy_invalid")).To(Equal("undetermined"), "gaps: %+v", doc.Gaps)
			Expect(nextActionKinds(doc)).NotTo(ContainElement("author_policy"))
			Expect(nextActionKinds(doc)).NotTo(ContainElement("restore_policy"))
		})
	})

	When("an explicit --project-config is a regular file on disk whose content differs from its committed copy, which is not valid", func() {
		It("passes the valid file on disk and discloses that it differs from HEAD", func() {
			commitFile(repo, "cfg/p.json", invalidPolicyText)
			Expect(os.WriteFile(filepath.Join(repo, "cfg", "p.json"), []byte(singleRootPolicyJSON), 0o644)).To(Succeed(), "the working tree holds a valid file; the analyzed revision decides its content")

			doc := expectCheckProject(repo, "--project-config", "cfg/p.json")

			Expect(doc.Checks.Policy.State).To(Equal("pass"), "gaps: %+v", doc.Gaps)
			Expect(policyDisclosureCodes(doc)).To(ConsistOf(differsWarning))
			Expect(nextActionKinds(doc)).NotTo(ContainElement("author_policy"))
			Expect(nextActionKinds(doc)).NotTo(ContainElement("restore_policy"))
		})
	})
})
