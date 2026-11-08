package main

import (
	"os"
	"path/filepath"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A committed policy that is gone from disk, with an edit staged for it, is brought back by one of two restores: the one that keeps
// the edit (git restore with no source) and the one that drops it (git restore --source=<revision> --staged --worktree). When the
// index entry carries a skip-worktree mark, whether set by hand or by a sparse checkout that leaves the path out, git refuses the
// first ("pathspec did not match") and runs the second, so the only printed command that worked would be the one that throws
// the edit away, and the user would loop. The mark is named first, the staged edit is mentioned in that message, and no restore is
// printed until the mark is cleared. Every step is run as the message says it.
var _ = Describe("coach codesignal: a committed policy gone from disk, with a staged edit, in an index entry that carries a mark", func() {
	const policy = ".lousy-agents/coach.json"
	const stagedEdit = `{"schema_version":"1","roots":["pkg"]}` + "\n"

	// stagedAndGone commits the policy, stages an edit to it, applies mark, and removes the file from disk.
	stagedAndGone := func(mark func(repo string)) string {
		repo := commitScannableGoRepo()
		commitFile(repo, "pkg/p.go", "package pkg\n")
		commitFile(repo, policy, singleRootPolicyJSON)
		writeWorktreeFile(repo, policy, stagedEdit)
		gitIn(repo, "add", "--", policy)
		mark(repo)
		_ = os.Remove(filepath.Join(repo, policy))
		return repo
	}
	keepCommand := func(stderr []byte) string {
		return regexp.MustCompile(`git restore -- '[^']+'`).FindString(string(stderr))
	}
	staysStaged := func(repo string) {
		content, err := os.ReadFile(filepath.Join(repo, policy))
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "the file is back")
		ExpectWithOffset(1, string(content)).To(Equal(stagedEdit), "the staged edit is what came back, not the committed copy")
	}

	It("prints no restore for a skip-worktree mark set by hand, and the commands that follow clearing it keep the edit", func() {
		repo := stagedAndGone(func(repo string) { gitIn(repo, "update-index", "--skip-worktree", "--", policy) })

		stderr := scanExplicitPolicy(repo, policy)

		Expect(string(stderr)).NotTo(ContainSubstring("git restore"), "git refuses the restore that keeps the edit and runs the one that drops it:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring("the index also holds a staged edit to it"), "stderr:\n%s", stderr)
		clear := printedClearMarkCommand(stderr)
		Expect(clear).NotTo(BeEmpty(), "the mark is what stops the restore, so the message names how to clear it:\n%s", stderr)
		output, exitCode := runInShell(repo, clear)
		Expect(exitCode).To(Equal(0), "command: %s\noutput: %s", clear, output)

		afterClearing := scanExplicitPolicy(repo, policy)
		keep := keepCommand(afterClearing)
		Expect(keep).NotTo(BeEmpty(), "with the mark gone the keep and the drop are offered:\n%s", afterClearing)
		output, exitCode = runInShell(repo, keep)
		Expect(exitCode).To(Equal(0), "the keep must work once the mark is cleared:\ncommand: %s\noutput: %s", keep, output)
		staysStaged(repo)
	})

	It("prints no restore when a cone sparse checkout leaves the path out, and widening the checkout writes the edit to the file", func() {
		repo := stagedAndGone(func(repo string) { gitIn(repo, "sparse-checkout", "set", "pkg") })

		stderr := scanExplicitPolicy(repo, policy)

		Expect(string(stderr)).NotTo(ContainSubstring("git restore"), "git refuses the restore that keeps the edit and runs the one that drops it:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring("the index also holds a staged edit to it"), "stderr:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring("add its directory to the sparse checkout"), "stderr:\n%s", stderr)
		gitIn(repo, "sparse-checkout", "add", ".lousy-agents")
		staysStaged(repo)
	})

	// The control: assume-unchanged is not a mark that stops a restore, so both restores are offered and the one that keeps the edit works.
	It("still offers the keep and the drop for a path marked assume-unchanged, and the keep works", func() {
		repo := stagedAndGone(func(repo string) { gitIn(repo, "update-index", "--assume-unchanged", "--", policy) })

		stderr := scanExplicitPolicy(repo, policy)

		keep := keepCommand(stderr)
		Expect(keep).NotTo(BeEmpty(), "stderr:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring("git restore --source="), "the drop is offered as well:\n%s", stderr)
		output, exitCode := runInShell(repo, keep)
		Expect(exitCode).To(Equal(0), "the keep must work:\ncommand: %s\noutput: %s", keep, output)
		staysStaged(repo)
	})
})
