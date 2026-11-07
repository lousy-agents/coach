package main

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// makeGitRecordIt is the step the advice names, for a rename that is yet to be made or is staged already.
func makeGitRecordIt(path string) string {
	return fmt.Sprintf("make git record it as %q (rename it in the repository, or commit a rename you have staged)", path)
}

// moveWhatStandsThereAside is the warning every rename advice carries, whatever the disk or git's settings are.
func moveWhatStandsThereAside(path string) string {
	return fmt.Sprintf("If another file already exists at %q, move it aside first; with core.ignorecase set, git can overwrite it", path)
}

const (
	readSpelling = ".lousy-agents/coach.json"
	oneRename    = `make git record it as ".lousy-agents/coach.json" (rename it in the repository, or commit a rename you have staged)`
	overwriting  = `If another file already exists at ".lousy-agents/coach.json", move it aside first; with core.ignorecase set, git can overwrite it`
)

// What the analyzed revision commits decides whether a policy is in the wrong case, and nothing else does: not the
// spelling a disk shows, not whether git ignores case, not what the index holds. So the refusal, its advice and its
// cause are the same for a policy Coach finds, one the user names, and the project check, on any disk, and the advice is
// told in words with no command to paste.
var _ = Describe("coach codesignal: a policy committed under other spellings than the one Coach reads", func() {
	type refusal struct {
		spellings []string
		says      []string
	}
	rows := []TableEntry{
		Entry("one other spelling", refusal{
			spellings: []string{".Lousy-Agents/Coach.json"},
			says: []string{
				`the policy is committed as ".Lousy-Agents/Coach.json", but Coach reads ".lousy-agents/coach.json", so it is not read: ` + oneRename + `, then commit and run again`,
				overwriting,
			},
		}),
		Entry("only the file in another case", refusal{
			spellings: []string{".lousy-agents/Coach.json"},
			says:      []string{`the policy is committed as ".lousy-agents/Coach.json", but Coach reads`, oneRename, overwriting},
		}),
		Entry("only the directory in another case", refusal{
			spellings: []string{".LOUSY-AGENTS/coach.json"},
			says:      []string{`the policy is committed as ".LOUSY-AGENTS/coach.json", but Coach reads`, oneRename, overwriting},
		}),
		Entry("two other spellings", refusal{
			spellings: []string{".LOUSY-AGENTS/coach.json", ".lousy-agents/Coach.json"},
			says: []string{
				`several spellings, ".LOUSY-AGENTS/coach.json", ".lousy-agents/Coach.json", and Coach reads ".lousy-agents/coach.json"`,
				`To keep ".lousy-agents/Coach.json", ` + oneRename + `, delete ".LOUSY-AGENTS/coach.json" from the repository, then commit and run again`,
				overwriting,
			},
		}),
		Entry("three other spellings", refusal{
			spellings: []string{".LOUSY-AGENTS/coach.json", ".Lousy-Agents/COACH.JSON", ".lousy-agents/Coach.json"},
			says: []string{
				`".LOUSY-AGENTS/coach.json", ".Lousy-Agents/COACH.JSON", ".lousy-agents/Coach.json"`,
				`To keep ".lousy-agents/Coach.json", ` + oneRename + `, delete ".LOUSY-AGENTS/coach.json", ".Lousy-Agents/COACH.JSON" from the repository, then commit and run again`,
				overwriting,
			},
		}),
	}

	scanDiscovered := func(repo string) string {
		GinkgoHelper()
		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--no-interactive")
		Expect(exitCode).To(Equal(2), "stdout: %s\nstderr: %s", stdout, stderr)
		Expect(string(stdout)).To(BeEmpty(), "no report beside a refusal")
		return string(stderr)
	}
	scanNamed := func(repo string) string {
		GinkgoHelper()
		return string(scanExplicitPolicy(repo, readSpelling))
	}
	expectTheProjectCheckAgrees := func(repo string, args ...string) {
		GinkgoHelper()
		doc := expectCheckProject(repo, args...)
		Expect(doc.Checks.Policy.Code).To(Equal("policy_invalid"), "gaps: %+v", doc.Gaps)
		Expect(causeOfGap(doc, "policy_invalid")).To(Equal("name_case_differs"), "gaps: %+v", doc.Gaps)
		_, authors := nextActionOfKind(doc, "author_policy")
		Expect(authors).To(BeFalse(), "a new policy cannot be written over the one that is committed: %+v", doc.NextActions)
	}

	DescribeTable("the scan refuses it, naming what is committed, in words",
		func(r refusal) {
			repo := commitScannableGoRepo()
			commitPolicyUnderSpellings(repo, r.spellings...)

			discovered, named := scanDiscovered(repo), scanNamed(repo)

			for _, message := range []string{discovered, named} {
				for _, want := range r.says {
					Expect(message).To(ContainSubstring(want), "stderr:\n%s", message)
				}
				Expect(message).To(ContainSubstring("(project_config_invalid)"))
				expectNoRunnableCommand(message)
			}
			Expect(named).To(Equal(discovered), "the policy Coach finds and the one the user names are told the same")
		},
		rows,
	)

	DescribeTable("the project check refuses it with the same cause and offers no new policy",
		func(r refusal) {
			repo := commitScannableGoRepo()
			commitPolicyUnderSpellings(repo, r.spellings...)

			expectTheProjectCheckAgrees(repo)
			expectTheProjectCheckAgrees(repo, "--project-config", readSpelling)
		},
		rows,
	)

	DescribeTable("it is the same wherever the file system and git stand on case",
		func(r refusal) {
			repo := commitScannableGoRepo()
			commitPolicyUnderSpellings(repo, r.spellings...)
			before := scanDiscovered(repo)

			gitIn(repo, "config", "core.ignorecase", "true")
			Expect(scanDiscovered(repo)).To(Equal(before), "with git set to ignore case")
			Expect(scanNamed(repo)).To(Equal(before), "named, with git set to ignore case")

			gitIn(repo, "config", "core.ignorecase", "false")
			Expect(scanDiscovered(repo)).To(Equal(before), "with git set to tell cases apart")
		},
		rows,
	)

	It("is no refusal when the revision commits the spelling Coach reads, whatever else it commits", func() {
		repo := commitScannableGoRepo()
		commitFile(repo, readSpelling, singleRootPolicyJSON)
		commitPolicyUnderSpellings(repo, readSpelling, ".LOUSY-AGENTS/coach.json")

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--no-interactive")

		Expect(exitCode).To(Equal(0), "stdout: %s\nstderr: %s", stdout, stderr)
		Expect(scanExplicitPolicyAccepted(repo, readSpelling)).To(ContainSubstring(`"schema_version":"2"`))
	})

	It("is no case refusal when the revision commits the policy in no spelling, and a new policy is still offered", func() {
		repo := commitScannableGoRepo()

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--no-interactive")

		Expect(exitCode).To(Equal(0), "stdout: %s\nstderr: %s", stdout, stderr)
		doc := expectCheckProject(repo)
		Expect(causeOfGap(doc, "policy_missing")).NotTo(Equal("name_case_differs"))
		_, authors := nextActionOfKind(doc, "author_policy")
		Expect(authors).To(BeTrue(), "next actions: %+v", doc.NextActions)
	})

	// Making git record the spelling Coach reads is the one way out, however the rename is made: until the commit,
	// the revision still holds the old spelling, so a rename that is staged is told what a rename that is not staged
	// is, and the advice already says to commit one.
	When("the user has made the rename and not yet committed it", func() {
		var repo string

		BeforeEach(func() {
			repo = commitScannableGoRepo()
			commitFile(repo, ".Lousy-Agents/Coach.json", singleRootPolicyJSON)
		})

		It("is told the same, in the same words, as before the rename", func() {
			before := scanDiscovered(repo)

			renameInTheRepository(repo, ".Lousy-Agents/Coach.json", readSpelling)

			Expect(scanDiscovered(repo)).To(Equal(before))
			Expect(scanNamed(repo)).To(Equal(before))
			expectTheProjectCheckAgrees(repo)
			expectTheProjectCheckAgrees(repo, "--project-config", readSpelling)
		})

		It("reads the policy once the rename is committed", func() {
			renameInTheRepository(repo, ".Lousy-Agents/Coach.json", readSpelling)
			gitInWithIdentity(repo, "commit", "-q", "-m", "make git record the policy as Coach reads it")

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--no-interactive")

			Expect(exitCode).To(Equal(0), "stdout: %s\nstderr: %s", stdout, stderr)
			Expect(string(stdout)).To(ContainSubstring(`"schema_version":"2"`))
			Expect(scanExplicitPolicyAccepted(repo, readSpelling)).To(ContainSubstring(`"schema_version":"2"`))
		})
	})
})
