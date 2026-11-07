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
	It("names not_committed and no new policy as the way out, in JSON and in text", func() {
		repo := commitScannableGoRepo()
		writeWorktreeFile(repo, ".lousy-agents/coach.json", singleRootPolicyJSON)

		doc := expectCheckProject(repo)
		stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, pathWithoutNode(), "--baseline", "--check-project", "--project-language", "typescript")

		Expect(causeOfGap(doc, "policy_missing")).To(Equal("not_committed"), "gaps: %+v", doc.Gaps)
		for _, action := range doc.NextActions {
			Expect(action.Kind).NotTo(Equal("author_policy"), "authoring refuses a target that exists: %+v", doc.NextActions)
		}
		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(string(stdout)).To(ContainSubstring("  policy_missing cause=not_committed\n"), "the text prints the same field:\n%s", stdout)
	})

	It("still offers a new policy where nothing is on disk", func() {
		repo := commitScannableGoRepo()

		doc := expectCheckProject(repo)

		Expect(causeOfGap(doc, "policy_missing")).To(Equal(""), "gaps: %+v", doc.Gaps)
		Expect(doc.NextActions).To(ContainElement(HaveField("Kind", "author_policy")), "next actions: %+v", doc.NextActions)
	})
})
