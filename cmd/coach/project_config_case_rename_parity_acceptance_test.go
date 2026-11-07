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

// aDirectoryInCase is the sentence that a rename of a directory in case only carries, because git cannot do it in one
// step on a disk that folds case.
const aDirectoryInCase = "A directory whose name changes only in case cannot be renamed in one step on a disk that folds case: rename it to another name first, then to the one you want."

const (
	readSpelling = ".lousy-agents/coach.json"
	oneRename    = `make git record it as ".lousy-agents/coach.json" (rename it in the repository, or commit a rename you have staged)`
	overwriting  = `If another file already exists at ".lousy-agents/coach.json", move it aside first; with core.ignorecase set, git can overwrite it`
)

// What the analyzed revision commits decides whether a policy is in the wrong case, and nothing else does: not the
// spelling a disk shows, not whether git ignores case, not what the index holds. So the refusal, its advice and its
// cause are the same for a policy Coach finds, one the user names, and the project check, on any disk, and the advice is
// told in words with no command to paste. A policy the user names has one more way out, which Coach's own path does not:
// to pass the spelling the revision commits.
var _ = Describe("coach codesignal: a policy committed under other spellings than the one Coach reads", func() {
	type refusal struct {
		spellings []string
		// says is in the message of the policy Coach finds and of the one the user names alike; saysNot is in neither.
		says, saysNot []string
		// discovered and named are in one of them only.
		discovered, named []string
	}
	rows := []TableEntry{
		Entry("one other spelling", refusal{
			spellings: []string{".Lousy-Agents/Coach.json"},
			says: []string{
				`the policy is committed as ".Lousy-Agents/Coach.json", but Coach reads ".lousy-agents/coach.json", so it is not read: `,
				oneRename + `, then commit and run again. ` + aDirectoryInCase + ` ` + overwriting,
			},
			discovered: []string{`so it is not read: ` + oneRename},
			named:      []string{`so it is not read: pass the committed spelling, ".Lousy-Agents/Coach.json", or ` + oneRename},
		}),
		Entry("only the file in another case", refusal{
			spellings:  []string{".lousy-agents/Coach.json"},
			says:       []string{`the policy is committed as ".lousy-agents/Coach.json", but Coach reads`, oneRename + `, then commit and run again. ` + overwriting},
			saysNot:    []string{aDirectoryInCase},
			discovered: []string{`so it is not read: ` + oneRename},
			named:      []string{`so it is not read: pass the committed spelling, ".lousy-agents/Coach.json", or ` + oneRename},
		}),
		Entry("only the directory in another case", refusal{
			spellings:  []string{".LOUSY-AGENTS/coach.json"},
			says:       []string{`the policy is committed as ".LOUSY-AGENTS/coach.json", but Coach reads`, oneRename, aDirectoryInCase, overwriting},
			discovered: []string{`so it is not read: ` + oneRename},
			named:      []string{`so it is not read: pass the committed spelling, ".LOUSY-AGENTS/coach.json", or ` + oneRename},
		}),
		Entry("two other spellings", refusal{
			spellings: []string{".LOUSY-AGENTS/coach.json", ".lousy-agents/Coach.json"},
			says: []string{
				`several spellings, ".LOUSY-AGENTS/coach.json", ".lousy-agents/Coach.json", and Coach reads ".lousy-agents/coach.json": `,
				`To keep ".lousy-agents/Coach.json", ` + oneRename + `, delete ".LOUSY-AGENTS/coach.json" from the repository, then commit and run again. ` + overwriting,
			},
			saysNot:    []string{aDirectoryInCase},
			discovered: []string{`Coach reads ".lousy-agents/coach.json": keep one of them at that path and remove the others.`},
			named:      []string{`Coach reads ".lousy-agents/coach.json": pass one of the committed spellings, or keep one of them at that path and remove the others.`},
		}),
		Entry("three other spellings", refusal{
			spellings: []string{".LOUSY-AGENTS/coach.json", ".Lousy-Agents/COACH.JSON", ".lousy-agents/Coach.json"},
			says: []string{
				`".LOUSY-AGENTS/coach.json", ".Lousy-Agents/COACH.JSON", ".lousy-agents/Coach.json"`,
				`To keep ".lousy-agents/Coach.json", ` + oneRename + `, delete ".LOUSY-AGENTS/coach.json", ".Lousy-Agents/COACH.JSON" from the repository, then commit and run again. ` + overwriting,
			},
			saysNot:    []string{aDirectoryInCase},
			discovered: []string{`keep one of them at that path and remove the others.`},
			named:      []string{`pass one of the committed spellings, or keep one of them at that path and remove the others.`},
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
				for _, unwanted := range r.saysNot {
					Expect(message).NotTo(ContainSubstring(unwanted), "stderr:\n%s", message)
				}
				Expect(message).To(ContainSubstring("(project_config_invalid)"))
				expectNoRunnableCommand(message)
			}
			for _, want := range r.discovered {
				Expect(discovered).To(ContainSubstring(want), "the policy Coach finds, stderr:\n%s", discovered)
			}
			for _, want := range r.named {
				Expect(named).To(ContainSubstring(want), "the policy the user names, stderr:\n%s", named)
			}
			Expect(discovered).NotTo(ContainSubstring("pass the committed spelling"), "Coach's own path is fixed, so renaming is the only way out:\n%s", discovered)
			Expect(discovered).NotTo(ContainSubstring("pass one of the committed spellings"), "Coach's own path is fixed:\n%s", discovered)
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
			before, namedBefore := scanDiscovered(repo), scanNamed(repo)

			gitIn(repo, "config", "core.ignorecase", "true")
			Expect(scanDiscovered(repo)).To(Equal(before), "with git set to ignore case")
			Expect(scanNamed(repo)).To(Equal(namedBefore), "named, with git set to ignore case")

			gitIn(repo, "config", "core.ignorecase", "false")
			Expect(scanDiscovered(repo)).To(Equal(before), "with git set to tell cases apart")
			Expect(scanNamed(repo)).To(Equal(namedBefore), "named, with git set to tell cases apart")
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
			before, namedBefore := scanDiscovered(repo), scanNamed(repo)

			renameInTheRepository(repo, ".Lousy-Agents/Coach.json", readSpelling)

			Expect(scanDiscovered(repo)).To(Equal(before))
			Expect(scanNamed(repo)).To(Equal(namedBefore))
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

// A name typed in another case than the revision commits is often a slip, and renaming the committed policy to match it
// would break everyone who passes the committed spelling, CI on a disk that tells cases apart included. So the words
// that a policy the user names gets offer passing the committed spelling beside the rename, and still print no command.
var _ = Describe("coach codesignal: a policy named in another case than the revision commits it", func() {
	var repo string

	BeforeEach(func() {
		repo = commitScannableGoRepo()
		commitFile(repo, "cfg/p.json", singleRootPolicyJSON)
	})

	DescribeTable("is told to pass the committed spelling or to make git record the one typed",
		func(typed, directoryNote string) {
			stderr := string(scanExplicitPolicy(repo, typed))

			Expect(stderr).To(ContainSubstring(`the policy is committed as "cfg/p.json", but Coach reads "`+typed+`", so it is not read: pass the committed spelling, "cfg/p.json", or `+makeGitRecordIt(typed)+`, then commit and run again.`), "stderr:\n%s", stderr)
			Expect(stderr).To(ContainSubstring(moveWhatStandsThereAside(typed)), "stderr:\n%s", stderr)
			if directoryNote == "" {
				Expect(stderr).NotTo(ContainSubstring(aDirectoryInCase), "stderr:\n%s", stderr)
			} else {
				Expect(stderr).To(ContainSubstring(directoryNote), "stderr:\n%s", stderr)
			}
			expectNoRunnableCommand(stderr)
		},
		Entry("a file typed in another case", "cfg/P.json", ""),
		Entry("a directory typed in another case", "CFG/p.json", aDirectoryInCase),
		Entry("a directory and a file typed in another case", "CFG/P.json", aDirectoryInCase),
	)

	It("reads the policy once the committed spelling is passed, and nothing is renamed", func() {
		Expect(scanExplicitPolicyAccepted(repo, "cfg/p.json")).To(ContainSubstring(`"schema_version":"2"`))
		Expect(gitOutputIn(repo, "ls-files")).To(ContainSubstring("cfg/p.json"))
	})

	It("is told the same by the project check, with the same cause and no new policy", func() {
		doc := expectCheckProject(repo, "--project-config", "CFG/p.json")

		Expect(causeOfGap(doc, "policy_invalid")).To(Equal("name_case_differs"), "gaps: %+v", doc.Gaps)
		_, authors := nextActionOfKind(doc, "author_policy")
		Expect(authors).To(BeFalse(), "next actions: %+v", doc.NextActions)
	})
})
