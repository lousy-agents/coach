package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal: an explicit policy that is on disk but not committed is read, not refused", func() {
	const uncommittedPolicy = "un.json"
	var repo string

	BeforeEach(func() {
		repo = commitScannableGoRepo()
		writeWorktreeFile(repo, uncommittedPolicy, singleRootPolicyJSON)
	})

	It("reads the file, exits 0, discloses it as not committed, and offers no new policy for a Go scan", func() {
		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", uncommittedPolicy, "--format=json")

		Expect(exitCode).To(Equal(0), "stdout=%s stderr=%s", stdout, stderr)
		Expect(scopePolicyScopeOf(stdout)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(uncommittedPolicy, singleRootPolicyJSON, diskCauseNotCommitted))))
		Expect(string(stderr)).NotTo(ContainSubstring(suggestCommand), "stderr:\n%s", stderr)
		text, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--project-config", uncommittedPolicy)
		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		expectDiskPolicyDisclosure(scopePolicyModes[0], text, diskPolicyLine(uncommittedPolicy, singleRootPolicyJSON, "not committed at HEAD."), diskPolicyNotCommittedWarning(uncommittedPolicy))
	})

	It("no longer refuses with the not-committed message, and offers no new policy, for a TypeScript scan", func() {
		args := []string{"codesignal", "--baseline", "--project-config", uncommittedPolicy, "--format=json", "--project-language", "typescript"}
		stdout, stderr, _ := runCoachBinary(commandPath, repo, stubToolchainEnv(pathWithStubNode("v24.9.9")), args...)

		Expect(string(stderr)).NotTo(ContainSubstring("exists in the worktree but is not committed"), "stdout=%s stderr=%s", stdout, stderr)
		Expect(string(stderr)).NotTo(ContainSubstring(suggestCommand), "stderr:\n%s", stderr)
	})

	It("names neither author_policy nor restore_policy as the next action of --check-project", func() {
		doc := expectCheckProject(repo, "--project-config", uncommittedPolicy)

		Expect(doc.Checks.Policy.Code).To(Equal("policy_missing"))
		Expect(nextActionKinds(doc)).NotTo(ContainElement("author_policy"))
		Expect(nextActionKinds(doc)).NotTo(ContainElement("restore_policy"))
	})
})

var _ = Describe("coach codesignal: an explicit policy over the size budget keeps the bound text and the offer of a smaller policy", func() {
	const oversizePolicy = "big.json"
	var repo string

	BeforeEach(func() {
		repo = commitScannableGoRepo()
		commitFile(repo, oversizePolicy, strings.Repeat(" ", 1<<20+1)+singleRootPolicyJSON)
	})

	It("names the bound that tripped and still offers a new policy", func() {
		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", oversizePolicy, "--format=json")

		Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring("is not readable at revision"))
		Expect(string(stderr)).To(ContainSubstring("git stdout exceeded"), "the text of the bound that tripped stays:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring(suggestCommand), "a smaller policy fixes a policy over the size budget:\n%s", stderr)
	})
})
