package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The default policy's path is the one a scan without --project-config reads. A policy named with that path in another
// case is refused when the disk lists the policy spelled as the default, with the spelling on disk to pass or to rename
// the file from.
var _ = Describe("coach codesignal: the default policy's path named in another case", func() {
	const (
		named  = ".lousy-agents/COACH.json"
		onDisk = ".lousy-agents/coach.json"
	)
	var repo string

	BeforeEach(func() {
		repo = commitScannableGoRepo()
		commitFile(repo, onDisk, singleRootPolicyJSON)
	})

	It("is told the spelling on disk to pass, or to rename the file on disk, and never to commit", func() {
		stderr := string(scanExplicitPolicy(repo, named))

		expectTheDiskSpellingRefusal(stderr)
		Expect(stderr).To(ContainSubstring(`the policy is on disk as ".lousy-agents/coach.json", but Coach reads ".lousy-agents/COACH.json", so it is not read: `), "stderr:\n%s", stderr)
		Expect(scanExplicitPolicyAccepted(repo, onDisk)).To(ContainSubstring(`"schema_version":"2"`))
	})

	It("is told the same by --check-project, and offers no new policy over the one that is committed", func() {
		doc := expectCheckProject(repo, "--project-config", named)

		Expect(causeOfGap(doc, "policy_invalid")).To(Equal("name_case_differs"), "gaps: %+v", doc.Gaps)
		_, authors := nextActionOfKind(doc, "author_policy")
		Expect(authors).To(BeFalse(), "next actions: %+v", doc.NextActions)
	})

})
