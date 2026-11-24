package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const readSpelling = ".lousy-agents/coach.json"

// What HEAD commits does not decide the spelling of the policy path: the directory listing on disk does, and
// the rows for it are in policy_spelling_on_disk_acceptance_test.go. A policy that HEAD commits under other
// spellings than the one on disk is disclosed, never refused.
var _ = Describe("coach codesignal: what HEAD commits beside the spelling Coach reads", func() {
	It("is no refusal when HEAD commits the spelling Coach reads, whatever else it commits", func() {
		repo := commitScannableGoRepo()
		commitFile(repo, readSpelling, singleRootPolicyJSON)
		commitPolicyUnderSpellings(repo, readSpelling, ".LOUSY-AGENTS/coach.json")

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--no-interactive")

		Expect(exitCode).To(Equal(0), "stdout: %s\nstderr: %s", stdout, stderr)
		Expect(scanExplicitPolicyAccepted(repo, readSpelling)).To(ContainSubstring(`"schema_version":"2"`))
	})

	It("is no case refusal when HEAD commits the policy in no spelling, and a new policy is still offered", func() {
		repo := commitScannableGoRepo()

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--no-interactive")

		Expect(exitCode).To(Equal(0), "stdout: %s\nstderr: %s", stdout, stderr)
		doc := expectCheckProject(repo)
		Expect(causeOfGap(doc, "policy_missing")).NotTo(Equal("name_case_differs"))
		_, authors := nextActionOfKind(doc, "author_policy")
		Expect(authors).To(BeTrue(), "next actions: %+v", doc.NextActions)
	})
})
