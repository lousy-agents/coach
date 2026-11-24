package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The documents describe the rule for a policy path spelled in another case: the disk decides the spelling, compared with
// the directory listing case-folded on every file system, and a spelling that only HEAD commits differently is disclosed.
// A paragraph that still says the analyzed revision decides, or says to commit, sends the user into the failure the
// disk rule was written to avoid.
const caseRuleStart = "The analyzed revision decides what a policy path names"

var _ = Describe("the CLI contract: the case rule it describes is the disk's", func() {
	var contract string

	BeforeEach(func() {
		contract = documentText("docs/cli-codesignal.md")
	})

	It("says the disk decides the spelling, compared case-folded on every file system", func() {
		Expect(contract).To(ContainSubstring("The disk decides the spelling of a policy path"))
		Expect(contract).To(ContainSubstring("case-folded, on every file system"))
		Expect(contract).To(ContainSubstring("An entry spelled exactly as the path wins over its case variants"))
	})

	It("says to rename the file on disk or to pass the spelling on disk, and lists up to 16 variants", func() {
		Expect(contract).To(ContainSubstring("rename the file on disk or pass the spelling on disk"))
		Expect(contract).To(ContainSubstring("up to 16"))
	})

	It("says a spelling that differs only from the committed one is disclosed, not refused", func() {
		Expect(contract).To(ContainSubstring("A spelling that differs only from the one committed at `HEAD` is disclosed"))
	})

	It("no longer says the analyzed revision decides what a policy path names", func() {
		Expect(contract).NotTo(ContainSubstring(caseRuleStart))
		Expect(contract).NotTo(ContainSubstring("make git record it at that path"))
	})

	It("names name_case_differs among the causes by what to do about it", func() {
		Expect(contract).To(ContainSubstring("`name_case_differs` (the disk lists the policy in another case than the path Coach reads; rename the file on disk or pass the spelling on disk)"))
	})
})

var _ = Describe("the CLI contract, ADR-008 and the PRD: the case rule they describe is the disk rule the CLI contract states", func() {
	It("tells the CLI contract's reader that the disk decides the spelling, in words, and never to commit", func() {
		bullet := cliContractBullet("A policy path that the disk lists in another case")
		text := strings.Join(strings.Fields(bullet), " ")

		Expect(text).To(ContainSubstring("`project_config_invalid`, and `--check-project` names `name_case_differs`"))
		Expect(text).To(ContainSubstring("rename the file on disk or pass the spelling on disk"))
		Expect(text).To(ContainSubstring("A spelling that differs only from the one committed at `HEAD` is disclosed with a WARNING, not refused"))
		Expect(text).NotTo(ContainSubstring("commit the"))
		expectNoRunnableCommand(bullet)
	})

	It("no longer tells the README's reader to record a rename with git mv, or that a policy Coach finds is moved by a printed command", func() {
		readme := documentText("README.md")

		Expect(readme).NotTo(ContainSubstring("is told to record the rename with"))
		Expect(readme).NotTo(ContainSubstring("the message prints the `git mv` that moves it"))
		Expect(readme).NotTo(ContainSubstring("whether to pass the revision's spelling"))
	})

	It("marks ADR-008's decision 3 as superseded by ADR-009's decision 5", func() {
		adr := documentText("docs/architecture/ADR-008-explicit-policy-disk-and-revision.md")

		Expect(adr).To(MatchRegexp(`3\. \*Which spelling does the path name\?\*.*[Ss]uperseded by \[?ADR-009`))
		Expect(adr).NotTo(ContainSubstring("The analyzed revision answers, and only it."))
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
