package main

import (
	"os"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// printedRestoreCommands are the git restore commands a stderr names, in the order it names them.
func printedRestoreCommands(stderr []byte) []string {
	return regexp.MustCompile(`git restore (?:--source=\S+ --staged --worktree )?-- '[^']+'`).FindAllString(string(stderr), -1)
}

// A committed policy with something staged over it that a policy cannot be, and gone from disk, is a state with two ways
// out: drop what is staged and bring back the committed copy, or keep what is staged. Keeping a symbolic link means
// writing the link back, which the scan refuses on the next run, so the message offers that step to nobody. Every command
// it does print is pasted here, because a command that looks right in the text is the one that sends the user round again.
var _ = Describe("coach codesignal: a committed policy with a symbolic link or a submodule entry staged over it and the file gone", func() {
	const policy = "cfg/p.json"

	stagedLink := func(repo string) {
		Expect(os.Remove(repo + "/" + policy)).To(Succeed())
		symlinkAt(repo, policy)
		gitIn(repo, "add", policy)
		removeTree(repo, "cfg")
	}
	stagedSubmoduleEntry := func(repo string) {
		gitIn(repo, "update-index", "--cacheinfo", "160000,"+strings.TrimSpace(gitIn(repo, "rev-parse", "HEAD"))+","+policy)
		removeTree(repo, "cfg")
	}
	committedPolicyRepo := func(stage func(repo string)) string {
		repo := commitScannableGoRepo()
		commitFile(repo, policy, singleRootPolicyJSON)
		stage(repo)
		return repo
	}

	It("prints a command that drops the staged link, and no command that brings it back: every command it prints ends the loop", func() {
		first := scanExplicitPolicy(committedPolicyRepo(stagedLink), policy)
		Expect(printedRestoreCommands(first)).NotTo(BeEmpty(), "the committed copy can be brought back, and the message says how:\n%s", first)

		for at := range printedRestoreCommands(first) {
			repo := committedPolicyRepo(stagedLink)
			command := printedRestoreCommands(scanExplicitPolicy(repo, policy))[at]

			output, exitCode := runInShell(repo, command)

			Expect(exitCode).To(Equal(0), "command: %s\noutput: %s", command, output)
			scanExplicitPolicyAccepted(repo, policy)
		}
	})

	It("prints no restore at all over a submodule entry, which the existing submodule message already withholds", func() {
		stderr := scanExplicitPolicy(committedPolicyRepo(stagedSubmoduleEntry), policy)

		Expect(printedRestoreCommands(stderr)).To(BeEmpty(), "a restore would write a regular file over a submodule entry the scan refuses:\n%s", stderr)
	})

	It("withholds the step that keeps the staged link and says why, naming the entry for a review", func() {
		stderr := scanExplicitPolicy(committedPolicyRepo(stagedLink), policy)

		Expect(string(stderr)).To(ContainSubstring("symbolic link"), "stderr:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring("that restore is not offered"), "stderr:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring("git diff --cached"), "stderr:\n%s", stderr)
		Expect(regexp.MustCompile(`git restore -- '[^']+'`).FindString(string(stderr))).To(BeEmpty(), "that command writes the link back, which the scan refuses:\n%s", stderr)
	})

	// Reviewing the staged entry changes nothing, so a step to run again after only that leaves the state as it was and
	// prints the same message: the user goes round. The only step that ends in running again is the one that changes the
	// index, which is staging a regular file where the link is.
	It("offers no step to run again that leaves the state unchanged, and names the regular file that keeps the change", func() {
		stderr := scanExplicitPolicy(committedPolicyRepo(stagedLink), policy)

		for _, clause := range strings.Split(string(stderr), "; ") {
			if strings.Contains(clause, "run again") {
				Expect(clause).To(ContainSubstring("stage a regular file at the path in place of the link"), "a step to run again that changes nothing:\n%s", stderr)
			}
		}
		Expect(string(stderr)).To(ContainSubstring("to keep the change, stage a regular file at the path in place of the link, then run again"), "stderr:\n%s", stderr)
	})

	It("ends the loop when a regular file is staged in place of the link, as the message says", func() {
		repo := committedPolicyRepo(stagedLink)
		scanExplicitPolicy(repo, policy)
		writeWorktreeFile(repo, policy, singleRootPolicyJSON)
		gitIn(repo, "add", policy)

		scanExplicitPolicyAccepted(repo, policy)
	})

	It("reports a submodule entry staged over it as in_submodule in --check-project, with the path left unlisted by that check", func() {
		repo := newTempGitRepo()
		commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
		commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
		commitFile(repo, policy, singleRootPolicyJSON)
		stagedSubmoduleEntry(repo)

		doc := expectCheckProject(repo, "--project-config", policy)

		Expect(causeOfGap(doc, "policy_missing")).To(Equal("in_submodule"))
	})

	It("names the cause and keeps the path in dirty_worktree in --check-project", func() {
		repo := newTempGitRepo()
		commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
		commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
		commitFile(repo, policy, singleRootPolicyJSON)
		stagedLink(repo)

		doc := expectCheckProject(repo, "--project-config", policy)

		Expect(causeOfGap(doc, "policy_missing")).To(Equal("staged_not_a_file"))
		Expect(doc.DirtyWorktree.RelevantChanges).To(BeTrue(), "the check did not read what the index holds")
		Expect(doc.DirtyWorktree.Paths).To(ContainElement(policy))
	})
})

var _ = Describe("the CLI contract: a link staged over a committed policy", func() {
	It("says the cause is staged_not_a_file, that only the restore that drops the staged link is printed, and what keeps the change", func() {
		text := documentText("docs/cli-codesignal.md")

		Expect(text).To(ContainSubstring("has the cause `staged_not_a_file`, and the message prints only the restore that drops the staged link, because the one that keeps it writes the link back and the scan refuses a link; to keep the change it says to stage a regular file at the path in place of the link, then run again."))
	})

	It("says a submodule entry staged over the policy is not that case but in_submodule, with no restore", func() {
		text := documentText("docs/cli-codesignal.md")

		Expect(text).To(ContainSubstring("A submodule entry (a gitlink) staged over the policy is not this case: it is reported as `in_submodule`, with no restore."))
	})
})
