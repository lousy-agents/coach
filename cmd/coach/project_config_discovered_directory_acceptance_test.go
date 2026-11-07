package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The canonical policy path can be committed as a directory. A scan and --check-project both read what the
// revision holds there before anything else, so both say it is a directory: the scan used to call the path
// "in the worktree but not committed" and go on without a policy, exit 0, while readiness reported
// policy_invalid.
var _ = Describe("coach codesignal: a discovered .lousy-agents/coach.json committed as a directory", func() {
	It("is refused by the scan, and --check-project reports the same cause", func() {
		repo := newTempGitRepo()
		commitFile(repo, "a.go", minimalGoSource)
		commitFile(repo, coachConfigPath+"/inner.json", singleRootPolicyJSON)

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json")
		doc := expectCheckProject(repo)

		Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring("committed as a directory"), "stderr:\n%s", stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("not committed"), "the path is committed:\n%s", stderr)
		Expect(causeOfGap(doc, "policy_invalid")).To(Equal("directory"), "gaps: %+v", doc.Gaps)
	})
})
