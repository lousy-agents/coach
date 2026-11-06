package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --check-project: a policy that is invalid at the revision names author_policy", func() {
	var repo string

	BeforeEach(func() {
		repo = newTempGitRepo()
		commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
		commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
	})

	When("the committed canonical policy is not valid", func() {
		It("reports policy_invalid and offers guided authoring, not a restore and not nothing", func() {
			commitFile(repo, coachConfigPath, invalidPolicyText)

			doc := expectCheckProject(repo)

			Expect(doc.Checks.Policy.Code).To(Equal("policy_invalid"))
			Expect(nextActionKinds(doc)).To(ContainElement("author_policy"), "a new policy is exactly the fix for a policy that cannot be parsed")
			Expect(nextActionKinds(doc)).NotTo(ContainElement("restore_policy"))
		})
	})

	When("an explicit --project-config is a regular file on disk but its committed copy is not valid", func() {
		It("reports policy_invalid and offers guided authoring", func() {
			commitFile(repo, "cfg/p.json", invalidPolicyText)
			Expect(os.WriteFile(filepath.Join(repo, "cfg", "p.json"), []byte(singleRootPolicyJSON), 0o644)).To(Succeed(), "the working tree holds a valid file; the analyzed revision decides its content")

			doc := expectCheckProject(repo, "--project-config", "cfg/p.json")

			Expect(doc.Checks.Policy.Code).To(Equal("policy_invalid"))
			Expect(nextActionKinds(doc)).To(ContainElement("author_policy"))
			Expect(nextActionKinds(doc)).NotTo(ContainElement("restore_policy"))
		})
	})
})
