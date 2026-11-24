package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The disk decides how a name spelled in another case is answered: the directory listing on disk, case-folded, on every
// file system, and never what the revision commits.
var _ = Describe("coach codesignal: the advice for a policy path spelled in another case follows what the disk lists", func() {
	var repo string

	BeforeEach(func() {
		repo = commitScannableGoRepo()
	})

	When("the disk lists the policy under a spelling other than the one the user typed", func() {
		BeforeEach(func() {
			commitFile(repo, "cfg/p.json", singleRootPolicyJSON)
		})

		DescribeTable("the user is told what is on disk, to rename the file on disk or to pass that spelling, and never to commit",
			func(typed string) {
				stderr := scanExplicitPolicy(repo, typed)

				expectTheDiskSpellingRefusal(string(stderr))
				Expect(string(stderr)).To(ContainSubstring(`the policy is on disk as "cfg/p.json", but Coach reads "`+typed+`", so it is not read: `), "stderr:\n%s", stderr)
			},
			Entry("a file typed in another case", "cfg/P.JSON"),
			Entry("a directory typed in another case", "CFG/p.json"),
			Entry("a directory and a file typed in another case", "CFG/P.json"),
		)

		It("takes the same cause in --check-project, and offers no new policy over the one that is committed", func() {
			doc := expectCheckProject(repo, "--project-config", "CFG/P.json")

			Expect(causeOfGap(doc, "policy_invalid")).To(Equal("name_case_differs"), "gaps: %+v", doc.Gaps)
			_, authors := nextActionOfKind(doc, "author_policy")
			Expect(authors).To(BeFalse(), "next actions: %+v", doc.NextActions)
		})
	})

	// The disk lists cfg/p.json, so a name typed as the revision commits it is refused when the disk spells it otherwise.
	When("the revision holds the spelling the user typed and the disk lists another", func() {
		BeforeEach(func() {
			commitFile(repo, "cfg/p.json", singleRootPolicyJSON)
		})

		It("refuses the name when only the file's case differs on disk, on every file system", func() {
			Expect(os.Rename(filepath.Join(repo, "cfg", "p.json"), filepath.Join(repo, "cfg", "P.JSON"))).To(Succeed())

			expectTheDiskSpellingRefusal(string(scanExplicitPolicy(repo, "cfg/p.json")))
		})

		It("refuses the name when only the directory's case differs on disk, on every file system", func() {
			Expect(os.Rename(filepath.Join(repo, "cfg"), filepath.Join(repo, "CFG"))).To(Succeed())

			expectTheDiskSpellingRefusal(string(scanExplicitPolicy(repo, "cfg/p.json")))
		})

		It("names name_case_differs in --check-project as well", func() {
			Expect(os.Rename(filepath.Join(repo, "cfg"), filepath.Join(repo, "CFG"))).To(Succeed())

			doc := expectCheckProject(repo, "--project-config", "cfg/p.json")

			Expect(causeOfGap(doc, "policy_invalid")).To(Equal("name_case_differs"), "gaps: %+v", doc.Gaps)
		})
	})

	When("the revision commits the policy in no spelling at all and the disk lists one", func() {
		It("refuses the name typed in another case, as for any file on disk", func() {
			Expect(os.MkdirAll(filepath.Join(repo, "cfg"), 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(repo, "cfg", "p.json"), []byte(singleRootPolicyJSON), 0o644)).To(Succeed())

			expectTheDiskSpellingRefusal(string(scanExplicitPolicy(repo, "cfg/P.JSON")))
		})
	})
})

// A spelling that only the revision commits is no spelling the disk lists, so the policy is missing as any absent path is.
var _ = Describe("coach codesignal: a policy missing from disk is not looked up in the spellings the revision holds", func() {
	var repo string

	BeforeEach(func() {
		repo = commitScannableGoRepo()
		commitFile(repo, "cfg/p.json", singleRootPolicyJSON)
	})

	It("calls a file name typed in another case missing, and does not say what is committed", func() {
		Expect(os.Remove(filepath.Join(repo, "cfg", "p.json"))).To(Succeed())

		stderr := scanExplicitPolicy(repo, "cfg/P.json")

		Expect(string(stderr)).To(ContainSubstring("project_config_missing"), "stderr:\n%s", stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("committed as"), "stderr:\n%s", stderr)
	})

	It("calls a directory name typed in another case missing, and does not say what is committed", func() {
		Expect(os.RemoveAll(filepath.Join(repo, "cfg"))).To(Succeed())

		stderr := scanExplicitPolicy(repo, "CFG/p.json")

		Expect(string(stderr)).To(ContainSubstring("project_config_missing"), "stderr:\n%s", stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("committed as"), "stderr:\n%s", stderr)
	})

	It("names no restore in --check-project for a file name typed in another case, because it is not committed at that path", func() {
		Expect(os.Remove(filepath.Join(repo, "cfg", "p.json"))).To(Succeed())

		doc := expectCheckProject(repo, "--project-config", "cfg/P.json")

		Expect(causeOfGap(doc, "policy_invalid")).To(Equal("no such gap"), "gaps: %+v", doc.Gaps)
		_, restores := nextActionOfKind(doc, "restore_policy")
		Expect(restores).To(BeFalse(), "a restore would write the wrong name: %+v", doc.NextActions)
	})

	It("offers the restore once the committed spelling is passed", func() {
		Expect(os.Remove(filepath.Join(repo, "cfg", "p.json"))).To(Succeed())

		stderr := scanExplicitPolicy(repo, "cfg/p.json")

		Expect(string(stderr)).To(ContainSubstring("git restore"), "stderr:\n%s", stderr)
	})

	It("still offers a new policy for a name the revision holds in no spelling", func() {
		stderr := scanExplicitPolicy(repo, "cfg/other.json")

		Expect(string(stderr)).To(ContainSubstring("suggest-project-config"), "stderr:\n%s", stderr)
	})
})
