package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// What the revision commits decides how a name spelled in another case is answered, and the disk is asked only
// whether a regular file is there: the same name is told the same on a disk that folds case and on one that does not.
var _ = Describe("coach codesignal: the advice for a policy path spelled in another case follows what the revision holds", func() {
	var repo string

	BeforeEach(func() {
		repo = commitScannableGoRepo()
	})

	When("the revision holds the policy under a spelling other than the one the user typed", func() {
		BeforeEach(func() {
			commitFile(repo, "cfg/p.json", singleRootPolicyJSON)
		})

		DescribeTable("the user is told what is committed, to pass that spelling or to make git record the one typed, in words",
			func(typed string) {
				stderr := scanExplicitPolicy(repo, typed)

				Expect(string(stderr)).To(ContainSubstring(`the policy is committed as "cfg/p.json", but Coach reads "`+typed+`", so it is not read: pass the committed spelling, "cfg/p.json", or `+makeGitRecordIt(typed)), "stderr:\n%s", stderr)
				Expect(string(stderr)).To(ContainSubstring(moveWhatStandsThereAside(typed)), "stderr:\n%s", stderr)
				Expect(string(stderr)).NotTo(ContainSubstring("pass --project-config"), "the way out is told in words, with no flag to paste:\n%s", stderr)
				expectNoRunnableCommand(string(stderr))
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

	// The revision decides what the path names, so a path it holds exactly as typed is read whatever spelling the
	// disk shows: git status is clean when only the case differs, and v0.11.0 read it.
	When("the revision holds the spelling the user typed and the disk shows another", func() {
		BeforeEach(func() {
			skipUnlessFilesystemResolvesCase()
			commitFile(repo, "cfg/p.json", singleRootPolicyJSON)
		})

		It("reads the policy from the revision when only the file's case differs on disk", func() {
			Expect(os.Rename(filepath.Join(repo, "cfg", "p.json"), filepath.Join(repo, "cfg", "P.JSON"))).To(Succeed())

			Expect(scanExplicitPolicyAccepted(repo, "cfg/p.json")).To(ContainSubstring(`"schema_version":"2"`))
		})

		It("reads the policy from the revision when only the directory's case differs on disk", func() {
			Expect(os.Rename(filepath.Join(repo, "cfg"), filepath.Join(repo, "CFG"))).To(Succeed())

			Expect(scanExplicitPolicyAccepted(repo, "cfg/p.json")).To(ContainSubstring(`"schema_version":"2"`))
		})

		It("reads the policy in --check-project as well", func() {
			Expect(os.Rename(filepath.Join(repo, "cfg"), filepath.Join(repo, "CFG"))).To(Succeed())

			doc := expectCheckProject(repo, "--project-config", "cfg/p.json")

			Expect(causeOfGap(doc, "policy_invalid")).To(Equal("no such gap"), "gaps: %+v", doc.Gaps)
			Expect(causeOfGap(doc, "policy_missing")).To(Equal("no such gap"), "gaps: %+v", doc.Gaps)
		})
	})

	When("the revision holds the policy in no spelling of the name typed", func() {
		It("gives no case advice, whatever the disk resolves the name to", func() {
			Expect(os.MkdirAll(filepath.Join(repo, "cfg"), 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(repo, "cfg", "p.json"), []byte(singleRootPolicyJSON), 0o644)).To(Succeed())

			stderr := scanExplicitPolicy(repo, "cfg/P.JSON")

			Expect(string(stderr)).NotTo(ContainSubstring("make git record it"), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("core.ignorecase"), "stderr:\n%s", stderr)
		})
	})
})

var _ = Describe("coach codesignal: a policy missing from disk is looked up in the spelling the revision holds", func() {
	var repo string

	BeforeEach(func() {
		repo = commitScannableGoRepo()
		commitFile(repo, "cfg/p.json", singleRootPolicyJSON)
	})

	It("refuses a file name typed in another case and offers no new policy, for a file", func() {
		Expect(os.Remove(filepath.Join(repo, "cfg", "p.json"))).To(Succeed())

		stderr := scanExplicitPolicy(repo, "cfg/P.json")

		Expect(string(stderr)).To(ContainSubstring(`committed as "cfg/p.json", but Coach reads "cfg/P.json"`), "stderr:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring("project_config_invalid"), "stderr:\n%s", stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("suggest-project-config"), "authoring there replaces the committed policy:\n%s", stderr)
	})

	It("refuses a directory name typed in another case and offers no new policy, for a directory", func() {
		Expect(os.RemoveAll(filepath.Join(repo, "cfg"))).To(Succeed())

		stderr := scanExplicitPolicy(repo, "CFG/p.json")

		Expect(string(stderr)).To(ContainSubstring(`committed as "cfg/p.json", but Coach reads "CFG/p.json"`), "stderr:\n%s", stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("suggest-project-config"), "stderr:\n%s", stderr)
	})

	It("names neither a restore nor a new policy in --check-project for a file name typed in another case", func() {
		Expect(os.Remove(filepath.Join(repo, "cfg", "p.json"))).To(Succeed())

		doc := expectCheckProject(repo, "--project-config", "cfg/P.json")

		_, restores := nextActionOfKind(doc, "restore_policy")
		_, authors := nextActionOfKind(doc, "author_policy")
		Expect(restores || authors).To(BeFalse(), "a restore would write the wrong name and a new policy would replace the committed one: %+v", doc.NextActions)
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
