package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A policy Coach would discover that is on disk and not committed gets one verdict from the scan and from the
// project check: the scan's diagnostic says the file is not committed so it was not read, and the check names the
// same cause and offers no new policy, because authoring refuses a target that exists. The way out is to commit
// the file, as it is for a policy named with --project-config.
var _ = Describe("coach codesignal --check-project: a discovered policy on disk that is not committed", func() {
	It("passes with no policy gap and no next action for the policy, and discloses that HEAD holds none", func() {
		repo := commitScannableGoRepo()
		writeWorktreeFile(repo, ".lousy-agents/coach.json", singleRootPolicyJSON)

		doc := expectCheckProject(repo)
		stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, pathWithoutNode(), "--baseline", "--check-project", "--project-language", "typescript")

		Expect(doc.Checks.Policy.State).To(Equal("pass"), "gaps: %+v", doc.Gaps)
		Expect(gapCodes(doc)).NotTo(ContainElement("policy_missing"))
		Expect(nextActionKinds(doc)).NotTo(ContainElement("author_policy"))
		Expect(policyDisclosureCodes(doc)).To(ConsistOf(notCommittedWarning))
		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(string(stdout)).NotTo(ContainSubstring("policy_missing"), "text:\n%s", stdout)
	})

	It("still offers a new policy where nothing is on disk", func() {
		repo := commitScannableGoRepo()

		doc := expectCheckProject(repo)

		Expect(causeOfGap(doc, "policy_missing")).To(Equal(""), "gaps: %+v", doc.Gaps)
		Expect(doc.NextActions).To(ContainElement(HaveField("Kind", "author_policy")), "next actions: %+v", doc.NextActions)
	})
})
