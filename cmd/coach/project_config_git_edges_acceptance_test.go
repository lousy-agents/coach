package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal: an explicit policy that is gone from disk is judged against git by the path as named", func() {
	var repo string

	BeforeEach(func() {
		repo = commitScannableGoRepo()
	})

	scanMissing := func(policy string) (stderr []byte) {
		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", policy, "--format=json")
		ExpectWithOffset(1, exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
		ExpectWithOffset(1, string(stderr)).To(ContainSubstring("project_config_missing"))
		return stderr
	}

	When("the path reads as pathspec magic for a file that is committed", func() {
		const magicPath = ":/a.go"

		It("does not take the committed file for the policy, and offers no restore command that cannot work", func() {
			stderr := scanMissing(magicPath)

			Expect(string(stderr)).NotTo(ContainSubstring("git restore"), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("committed at the analyzed revision"), "no file is committed at that path:\n%s", stderr)
		})

		It("reports no restore_policy next action in --check-project", func() {
			doc := expectCheckProject(repo, "--project-config", magicPath)

			Expect(doc.Checks.Policy.Code).To(Equal("policy_missing"))
			Expect(nextActionKinds(doc)).NotTo(ContainElement("restore_policy"))
		})
	})

	When("the policy is committed but left out of the sparse checkout", func() {
		const sparsePolicy = "cfg/p.json"

		BeforeEach(func() {
			commitFile(repo, sparsePolicy, singleRootPolicyJSON)
			gitIn(repo, "sparse-checkout", "set", "--no-cone", "/*", "!/cfg/")
		})

		It("offers no restore command, which git rejects for a path the sparse checkout skips, and says why", func() {
			stderr := scanMissing(sparsePolicy)

			Expect(string(stderr)).NotTo(ContainSubstring("git restore"), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring(suggestCommand), "stderr:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring("sparse checkout"), "the message names why no restore is offered:\n%s", stderr)
		})

		It("names neither restore_policy nor author_policy in --check-project", func() {
			doc := expectCheckProject(repo, "--project-config", sparsePolicy)

			Expect(doc.Checks.Policy.Code).To(Equal("policy_missing"))
			Expect(nextActionKinds(doc)).NotTo(ContainElement("restore_policy"))
			Expect(nextActionKinds(doc)).NotTo(ContainElement("author_policy"))
		})
	})
})
