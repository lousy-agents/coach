package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func documentText(relative string) string {
	GinkgoHelper()
	text, err := os.ReadFile(filepath.Join("..", "..", relative))
	Expect(err).NotTo(HaveOccurred())
	return strings.Join(strings.Fields(string(text)), " ")
}

// A committed policy that is gone from disk, with an edit to it staged, has two restores and Coach prints both: the
// restore from the index, which keeps the edit, and the restore from the revision, which brings the committed copy
// back and drops the edit. The second is not the default and is labelled, and the message must not say that no
// restore is offered while it prints one. The README, the PRD and the ADR describe the same two choices.
var _ = Describe("coach codesignal: a committed policy with a staged edit, gone from disk, has a keep choice and a labelled drop choice", func() {
	const (
		policy       = "pol.json"
		dropPhrase   = "to drop it and bring back the committed copy"
		stagedPolicy = `{"schema_version":"1","roots":["staged"]}`
	)

	It("does not say none is offered while it prints the restore that drops the edit", func() {
		repo := commitScannableGoRepo()
		commitFile(repo, policy, singleRootPolicyJSON)
		Expect(os.WriteFile(filepath.Join(repo, policy), []byte(stagedPolicy), 0o644)).To(Succeed())
		gitIn(repo, "add", policy)
		Expect(os.Remove(filepath.Join(repo, policy))).To(Succeed())

		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", policy, "--format=json", "--no-interactive")

		Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("so none is offered"), "the message prints a restore, so it cannot say none is offered:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring("a restore from the revision would discard, so it is not offered as the default"), "stderr:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring("to keep it as it is"), "stderr:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring(dropPhrase+" instead, restore it with git restore --source="), "the drop is labelled and printed:\n%s", stderr)
	})

	It("is described by the README as two choices, and not as no restore", func() {
		bullet := readmeBullet("Nothing is there but the policy is committed at the analyzed revision")

		Expect(bullet).To(ContainSubstring(dropPhrase))
		Expect(bullet).NotTo(ContainSubstring("a staged edit to the policy or an unmerged entry for it"), "a staged edit is no longer grouped with the cases that get no restore")
	})

	It("is described by the PRD as two choices, and not as no restore command", func() {
		prd := documentText("docs/product/prd.md")

		Expect(prd).To(ContainSubstring(dropPhrase))
		Expect(prd).NotTo(ContainSubstring("where running it would overwrite something or recreate what the scan refuses: a staged edit to the policy"))
	})

	It("is described by the CLI reference as two choices", func() {
		Expect(documentText("docs/cli-codesignal.md")).To(ContainSubstring("as the labelled way to drop it, the restore from the revision"))
	})

	It("is described by ADR-008 as two choices, and not as no restore", func() {
		adr := documentText("docs/architecture/ADR-008-explicit-policy-disk-and-revision.md")

		Expect(adr).To(ContainSubstring(dropPhrase))
		Expect(adr).NotTo(ContainSubstring("and no restore is offered over it"))
	})
})
