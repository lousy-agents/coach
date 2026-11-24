package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The disk decides the spelling, so a policy gone from disk is not a case problem whatever spelling HEAD commits it
// under. This pins the boundary between the disk spelling walk and the discovery fallback (#168 A3): a discovered
// policy that HEAD commits only in another case and that is not on disk in any spelling is no policy at the default path.
var _ = Describe("coach codesignal: a policy missing on disk that HEAD commits in another case", func() {
	arrange := func() (repo, first string) {
		repo, first = scannableRepoWithTwoCommits()
		commitFile(repo, ".Lousy-Agents/Coach.json", singleRootPolicyJSON)
		Expect(os.RemoveAll(filepath.Join(repo, ".Lousy-Agents"))).To(Succeed())
		return repo, first
	}

	It("is no case refusal for a scan: it goes on without a policy", func() {
		repo, _ := arrange()

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--no-interactive")

		Expect(exitCode).To(Equal(0), "stdout: %s\nstderr: %s", stdout, stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("name_case_differs"))
		Expect(string(stdout)).NotTo(ContainSubstring("name_case_differs"))
	})

	It("is a missing policy in --check-project, with no name_case_differs cause", func() {
		repo, _ := arrange()

		doc, _ := checkProjectBothFormats(repo, pathWithoutNode())

		Expect(doc.Checks.Policy.Code).To(Equal("policy_missing"))
		Expect(doc.Gaps).NotTo(ContainElement(HaveField("Cause", "name_case_differs")))
	})
})
