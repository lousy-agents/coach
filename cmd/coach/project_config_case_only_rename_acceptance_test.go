package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// renameInCaseOnly renames through a temporary name, which a file system that ignores case needs and one
// that does not accepts.
func renameInCaseOnly(from, to string) {
	ExpectWithOffset(1, os.Rename(from, from+".tmp")).To(Succeed())
	ExpectWithOffset(1, os.Rename(from+".tmp", to)).To(Succeed())
}

// The disk decides the spelling, on every file system: a policy renamed in case only is the file at the spelling the
// disk now lists, and the revision, which still commits the old spelling, is disclosed and does not refuse it.
var _ = Describe("coach codesignal: a policy renamed in case only on disk", func() {
	var repo string

	BeforeEach(func() {
		repo = commitScannableGoRepo()
		commitFile(repo, "policies/p.json", singleRootPolicyJSON)
	})

	When("the file was renamed on disk and nothing was recorded", func() {
		BeforeEach(func() {
			renameInCaseOnly(filepath.Join(repo, "policies", "p.json"), filepath.Join(repo, "policies", "P.json"))
		})

		It("reads the policy at the spelling the disk lists and discloses that HEAD commits another", func() {
			Expect(scanExplicitPolicyAccepted(repo, "policies/P.json")).To(ContainSubstring(`"cause":"not_committed"`))
		})

		It("refuses the spelling HEAD commits, and says to rename the file on disk or to pass the spelling on disk", func() {
			stderr := scanExplicitPolicy(repo, "policies/p.json")

			expectTheDiskSpellingRefusal(string(stderr))
			Expect(string(stderr)).To(ContainSubstring(`the policy is on disk as "policies/P.json", but Coach reads "policies/p.json"`), "stderr:\n%s", stderr)
		})

		It("passes --check-project for the spelling on disk, with the disclosure, and names name_case_differs for the committed one", func() {
			doc := expectCheckProject(repo, "--project-config", "policies/P.json")
			Expect(causeOfGap(doc, "policy_invalid")).To(Equal("no such gap"), "gaps: %+v", doc.Gaps)
			Expect(policyDisclosureCodes(doc)).To(ConsistOf(notCommittedWarning))

			refused := expectCheckProject(repo, "--project-config", "policies/p.json")
			Expect(causeOfGap(refused, "policy_invalid")).To(Equal("name_case_differs"), "gaps: %+v", refused.Gaps)
		})
	})

	When("the directory was renamed on disk", func() {
		BeforeEach(func() {
			renameInCaseOnly(filepath.Join(repo, "policies"), filepath.Join(repo, "Policies"))
		})

		It("reads the policy at the directory spelling the disk lists, and refuses the one HEAD commits", func() {
			Expect(scanExplicitPolicyAccepted(repo, "Policies/p.json")).To(ContainSubstring(`"cause":"not_committed"`))

			expectTheDiskSpellingRefusal(string(scanExplicitPolicy(repo, "policies/p.json")))
		})
	})
})
