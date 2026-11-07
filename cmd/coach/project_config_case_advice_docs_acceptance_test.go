package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The documents describe the rule for a policy path spelled in another case: the analyzed revision decides what a path
// names, with one message that describes the rename in words, for a policy that is named and one that Coach finds, on any
// file system. The cmd/coach acceptance specs for the name-case advice hold the scan to that rule; these hold the documents to
// it. A paragraph that still promises a path to pass, a correction one component per run, or a command to run, sends the
// user into the failure the described step was written to avoid.
const (
	caseRuleStart = "The analyzed revision decides what a policy path names"
	caseRuleEnd   = "Only the policy's own path leaves `dirty_worktree`."
)

// theCaseRule is the part of the CLI contract that describes the rule.
func theCaseRule() string {
	GinkgoHelper()
	text := documentText("docs/cli-codesignal.md")
	start, end := strings.Index(text, caseRuleStart), strings.Index(text, caseRuleEnd)
	Expect(start).To(BeNumerically(">=", 0), "the CLI contract no longer has the paragraph that begins %q", caseRuleStart)
	Expect(end).To(BeNumerically(">", start), "the CLI contract no longer ends the case rule before %q", caseRuleEnd)
	return text[start:end]
}

var _ = Describe("the CLI contract: the case rule it describes is the one the scan applies", func() {
	var rule string

	BeforeEach(func() {
		rule = theCaseRule()
	})

	It("says the revision decides what a path names, and the disk is asked only whether a regular file is there", func() {
		Expect(rule).To(ContainSubstring("The analyzed revision decides what a policy path names, for a policy named with `--project-config` and for the one Coach finds alike."))
		Expect(rule).To(ContainSubstring("When the revision commits exactly the path as written, that is the policy that is read, whatever spelling the disk shows for the file and whatever `core.ignorecase` says"))
		Expect(rule).To(ContainSubstring("The disk is asked only whether a regular file is at the path"))
	})

	It("says one message serves a named policy and a discovered one, on any file system", func() {
		Expect(rule).To(ContainSubstring("One message serves a policy named with `--project-config` and a policy Coach finds, in a scan and in `--check-project`, on a file system that folds case and on one that tells cases apart, and whatever `core.ignorecase` says"))
		Expect(rule).To(ContainSubstring("`--check-project` reports `policy_invalid` with the cause `name_case_differs` and offers no `author_policy`"))
	})

	It("says the step is told in words with no command, with the warning about a file already at the destination", func() {
		Expect(rule).To(ContainSubstring("It states the step in words and prints no command."))
		Expect(rule).To(ContainSubstring("It always warns that if another file already exists at that path it is to be moved aside first, because with `core.ignorecase` set git can overwrite it."))
		expectNoRunnableCommand(rule)
	})

	It("says a rename that is already staged is told the same as one that is not", func() {
		Expect(rule).To(ContainSubstring("A rename that is already staged is told the same as one that is not, on every file system, because the revision still holds the old spelling until the rename is committed."))
	})

	It("says a policy the user names is also offered the committed spelling, and Coach's own path is not", func() {
		Expect(rule).To(ContainSubstring("A policy named with `--project-config` has one more way out, because a name typed in another case is often a slip and renaming the committed policy to match it would break everyone who passes the committed spelling: the message first says to pass the committed spelling, or one of them when several are committed, and then says to make git record the policy at the name given."))
		Expect(rule).To(ContainSubstring("The path Coach finds has one spelling it can be read at, so for it renaming is the only way out and the message does not offer to pass anything."))
	})

	It("says a rename that changes a directory's name in case only takes two steps on a file system that folds case", func() {
		Expect(rule).To(ContainSubstring("When the rename changes the case of a directory's name, the message adds that git cannot do that in one step on a file system that folds case, and says to rename the directory to another name first and then to the one wanted."))
	})

	It("says the default policy's path named in another case is never told to be recorded at that spelling", func() {
		Expect(rule).To(ContainSubstring("A policy named with the default policy's path in another case (`.lousy-agents/COACH.json`) is not told to make git record the policy at the spelling typed, because a scan without `--project-config` reads only `.lousy-agents/coach.json` and would then refuse it."))
		Expect(rule).To(ContainSubstring("It is told to pass the committed spelling or to leave the flag off; when the revision commits the default spelling beside others it is told to keep that one and delete the others, and the default spelling is never the one to delete; when only another spelling is committed, git is to record the policy at the default spelling."))
	})

	It("says what is listed when the revision commits several spellings, and the limit of 16", func() {
		Expect(rule).To(ContainSubstring("the message lists every one of them (up to 16, then \"and more\"), says to keep one of them at that path and remove the others, names the one to keep and the ones to delete, and gives the same warning; past 16 spellings it names none to keep, and says only to keep one of them at that path and remove the others"))
	})

	It("says nothing is refused when the revision commits the path as written or nothing like it", func() {
		Expect(rule).To(ContainSubstring("Nothing is refused when the revision commits the path as written, whatever else it commits beside it, or when it commits nothing like it."))
		Expect(rule).To(ContainSubstring("a missing policy under a parent typed in another case that the revision holds no spelling of is simply missing, and is not refused for the parent"))
	})

	It("says a policy gone from disk gets the same message, with no restore and no new policy", func() {
		Expect(rule).To(ContainSubstring("A policy that is gone from disk while the revision commits it in another case gets the same message, and no restore and no new policy is offered, because either would write into the other spelling."))
	})

	It("says a file only the worktree has, in no committed spelling, is not a case problem", func() {
		Expect(rule).To(ContainSubstring("A file in the worktree that the revision commits in no spelling at all is not a case problem: on a file system that folds case it is the file at the path and is not committed (`not_committed`)"))
		Expect(rule).To(ContainSubstring("on a file system that tells cases apart a file spelled in another case is not at the path, so the policy is missing and `author_policy` is offered"))
	})

	It("says the discovered policy is refused with the same message", func() {
		Expect(rule).To(ContainSubstring("A policy that Coach discovers, with no `--project-config`, is read only at `.lousy-agents/coach.json`."))
		Expect(rule).To(ContainSubstring("the scan is refused with the message above, and `--check-project` reports a policy gap with the cause `name_case_differs` and no `author_policy`"))
	})

	It("no longer promises a path to pass, a correction one component per run, or a move to run", func() {
		text := documentText("docs/cli-codesignal.md")
		for _, stale := range []string{"one component per run", "correcting the first leads to the message for the next", "the pass clause", "pass --project-config cfg/p.json", "record the rename with", "the revision holds neither spelling", "stored as"} {
			Expect(text).NotTo(ContainSubstring(stale), "the CLI contract still carries %q from a rule that is gone", stale)
		}
	})

	It("names name_case_differs among the causes by what to do about it", func() {
		Expect(documentText("docs/cli-codesignal.md")).To(ContainSubstring("`name_case_differs` (the revision commits the policy in another case than the path Coach reads; make git record it at that path)"))
	})
})

var _ = Describe("the README, ADR-008 and the PRD: the case rule they describe is the one the CLI contract states", func() {
	It("tells the README's reader the one message, in words, with the warning, for a staged rename as for any other", func() {
		bullet := readmeBullet("A policy path that the analyzed revision does not commit as written")
		text := strings.Join(strings.Fields(bullet), " ")

		Expect(text).To(ContainSubstring("`project_config_invalid`, and `--check-project` names `name_case_differs` and offers no new policy"))
		Expect(text).To(ContainSubstring("One message serves a policy you name and one Coach finds, on any file system and whatever `core.ignorecase` says"))
		Expect(text).To(ContainSubstring("says in words to make git record the policy at that path, then commit and run again. It prints no command"))
		Expect(text).To(ContainSubstring("it always warns that a file already at that path is to be moved aside first, because git can overwrite it with `core.ignorecase` set"))
		Expect(text).To(ContainSubstring("A rename you have already staged is told the same as one you have not"))
		Expect(text).To(ContainSubstring("A policy you name is also told to pass the committed spelling, which is the way out for a slip of the case, and a directory renamed in case only is told to take two steps on a file system that folds case"))
		expectNoRunnableCommand(bullet)
	})

	It("no longer tells the README's reader to record a rename with git mv, or that a policy Coach finds is moved by a printed command", func() {
		readme := documentText("README.md")

		Expect(readme).NotTo(ContainSubstring("is told to record the rename with"))
		Expect(readme).NotTo(ContainSubstring("the message prints the `git mv` that moves it"))
		Expect(readme).NotTo(ContainSubstring("whether to pass the revision's spelling"))
	})

	It("amends ADR-008: the revision decides which spelling a path names, and the disk is not asked", func() {
		adr := documentText("docs/architecture/ADR-008-explicit-policy-disk-and-revision.md")

		Expect(adr).To(ContainSubstring("3. *Which spelling does the path name?* The analyzed revision answers, and only it."))
		Expect(adr).To(ContainSubstring("An earlier slice of this work also compared the spelling the disk showed with the one written, which made the answer depend on whether the file system folds case; that check is removed."))
		Expect(adr).To(ContainSubstring("The message names the committed spelling and the path Coach reads and says in words to make git record the policy at that path; it prints no command."))
		Expect(adr).To(ContainSubstring("A policy that is named is also offered the other way out, which is to pass the committed spelling, because the name may be a slip and renaming the committed policy would break the callers that pass it; the path discovery finds has one spelling and is offered only the rename."))
		Expect(adr).NotTo(ContainSubstring("both naming the `git mv` that moves it"))
	})

	It("marks the checklist rows that recorded the old case behaviour as superseded, and not as the current rule", func() {
		raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "development", "evil-input-checklist-595.md"))
		Expect(err).NotTo(HaveOccurred())
		rows := strings.Split(string(raw), "\n")

		for _, prefix := range []string{"| 11 | A file renamed in case only", "| 12 | A discovered policy committed in another case", "| N16 | case collision", "| N17 | policy.json exists on disk only in another case"} {
			index := slices.IndexFunc(rows, func(row string) bool { return strings.HasPrefix(row, prefix) })
			Expect(index).To(BeNumerically(">=", 0), "the checklist no longer has the row that begins %q", prefix)
			Expect(rows[index]).To(ContainSubstring("uperseded"), "the row that begins %q describes the old case behaviour and says it is superseded", prefix)
		}
	})

	It("requires in the PRD that the rename is described in words, with the warning, and the same for a staged rename", func() {
		prd := documentText("docs/product/prd.md")

		Expect(prd).To(ContainSubstring("WHERE the analyzed revision does not commit a policy path as written but commits the same path in another case, THE Coach CLI shall exit with status `2`, name `project_config_invalid` and the cause `name_case_differs`"))
		Expect(prd).To(ContainSubstring("It shall print no `git mv`, `git rm` or other shell command, shall always warn that a file already at the destination is to be moved aside first because git can overwrite it when `core.ignorecase` is set, and shall tell a rename that is already staged the same as one that is not."))
		Expect(prd).To(ContainSubstring("The message shall be the same for a policy named with `--project-config` and a policy Coach finds, on a file system that folds case and one that does not, except that for a policy named with `--project-config` it shall first offer passing the committed spelling, or one of them when several are committed, because the name may be a slip. WHERE the rename changes the case of a directory's name, it shall add that git cannot do that in one step on a file system that folds case, and say to rename the directory to another name first."))
	})
})
