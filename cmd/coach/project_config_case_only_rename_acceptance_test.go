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

// What the disk shows for a name is not asked: a file system that ignores case resolves a policy renamed in case only
// to the file the user means, and the analyzed revision, which still commits the old spelling, is what refuses it, in
// the words it uses for a policy committed in another case on any disk.
var _ = Describe("coach codesignal: a policy renamed in case only, on a file system that ignores case", func() {
	var repo string

	BeforeEach(func() {
		skipUnlessFilesystemResolvesCase()
		repo = commitScannableGoRepo()
		commitFile(repo, "policies/p.json", singleRootPolicyJSON)
	})

	expectedToMakeGitRecord := func(stderr []byte, committed, read string) {
		GinkgoHelper()
		Expect(string(stderr)).To(ContainSubstring(`the policy is committed as "`+committed+`", but Coach reads "`+read+`", so it is not read: `+makeGitRecordIt(read)), "stderr:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring(moveWhatStandsThereAside(read)), "stderr:\n%s", stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("commit the file so it is readable"), "committing the file records nothing here:\n%s", stderr)
		expectNoRunnableCommand(string(stderr))
	}

	When("the file was renamed on disk and nothing was recorded", func() {
		BeforeEach(func() {
			renameInCaseOnly(filepath.Join(repo, "policies", "p.json"), filepath.Join(repo, "policies", "P.json"))
		})

		It("says the revision holds the old spelling and to make git record the new one", func() {
			expectedToMakeGitRecord(scanExplicitPolicy(repo, "policies/P.json"), "policies/p.json", "policies/P.json")
		})

		It("names name_case_differs in --check-project, not not_committed, and offers no new policy", func() {
			doc := expectCheckProject(repo, "--project-config", "policies/P.json")

			Expect(causeOfGap(doc, "policy_invalid")).To(Equal("name_case_differs"), "gaps: %+v", doc.Gaps)
			_, authors := nextActionOfKind(doc, "author_policy")
			Expect(authors).To(BeFalse(), "next actions: %+v", doc.NextActions)
		})

		It("lets the scan read the policy once the rename the message describes has been made and committed", func() {
			gitIn(repo, "mv", "--", "policies/p.json", "policies/P.json")
			gitInWithIdentity(repo, "commit", "-q", "-m", "rename the policy")

			Expect(scanExplicitPolicyAccepted(repo, "policies/P.json")).To(ContainSubstring(`"schema_version":"2"`))
		})
	})

	When("the directory was renamed on disk", func() {
		BeforeEach(func() {
			renameInCaseOnly(filepath.Join(repo, "policies"), filepath.Join(repo, "Policies"))
		})

		It("says the revision holds the old spelling and to make git record the new one, as for a file", func() {
			expectedToMakeGitRecord(scanExplicitPolicy(repo, "Policies/p.json"), "policies/p.json", "Policies/p.json")
		})
	})
})
