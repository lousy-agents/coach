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

	When("an explicit --project-config is a regular file on disk and is the committed copy, which is not valid", func() {
		It("reports policy_invalid and offers guided authoring, because there is no work in the file to lose", func() {
			commitFile(repo, "cfg/p.json", invalidPolicyText)

			doc := expectCheckProject(repo, "--project-config", "cfg/p.json")

			Expect(doc.Checks.Policy.Code).To(Equal("policy_invalid"))
			Expect(causeOfGap(doc, "policy_invalid")).To(BeEmpty(), "a gap with a way out needs no cause: %+v", doc.Gaps)
			Expect(nextActionKinds(doc)).To(ContainElement("author_policy"))
			Expect(nextActionKinds(doc)).NotTo(ContainElement("restore_policy"))
		})
	})

	// The working tree holds a valid file where the committed copy is not valid. The analyzed revision still decides
	// what the policy is, so the gap is policy_invalid; but the file on disk is work, and a replacement would discard
	// it, so no authoring is offered, the cause says that the file on disk differs, and the way out is to commit it.
	When("an explicit --project-config is a regular file on disk whose content differs from its committed copy, which is not valid", func() {
		It("reports policy_invalid with the cause changed_on_disk and offers no authoring, no restore", func() {
			commitFile(repo, "cfg/p.json", invalidPolicyText)
			Expect(os.WriteFile(filepath.Join(repo, "cfg", "p.json"), []byte(singleRootPolicyJSON), 0o644)).To(Succeed(), "the working tree holds a valid file; the analyzed revision decides its content")

			doc := expectCheckProject(repo, "--project-config", "cfg/p.json")

			Expect(doc.Checks.Policy.Code).To(Equal("policy_invalid"))
			Expect(causeOfGap(doc, "policy_invalid")).To(Equal("changed_on_disk"))
			Expect(nextActionKinds(doc)).NotTo(ContainElement("author_policy"), "a replacement would discard the file on disk")
			Expect(nextActionKinds(doc)).NotTo(ContainElement("restore_policy"))
		})
	})
})
