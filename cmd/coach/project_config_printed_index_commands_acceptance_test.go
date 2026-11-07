package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// printedClearMarkCommand is the command a stderr prints to clear a skip-worktree mark: the words after "clear the
// mark with", up to the ", then run again" that closes the sentence, or "" when none is printed.
func printedClearMarkCommand(stderr []byte) string {
	for _, line := range strings.Split(string(stderr), "\n") {
		if _, command, found := strings.Cut(line, "clear the mark with "); found {
			command, _, _ = strings.Cut(command, ", then run again")
			return strings.TrimSpace(command)
		}
	}
	return ""
}

// The two commands a missing policy's message prints that work on the index (clearing a
// skip-worktree mark, and restoring a staged copy from the index) are run as printed, because a
// command that git rejects, or that reads the name as a glob or as the wrong directory's file, looks
// the same as a good one in the text. The names carry glob characters and a space for that reason.
var _ = Describe("coach codesignal: the commands printed for a policy the index holds, run as printed", func() {
	policyNames := []string{"cfg/p.json", "cfg/a[1]*.json", "cfg/my policy.json"}

	deleteFromDisk := func(repo, policy string) {
		Expect(os.RemoveAll(filepath.Join(repo, policy))).To(Succeed())
	}
	stageWithoutCommitting := func(repo, policy string) {
		Expect(os.MkdirAll(filepath.Dir(filepath.Join(repo, policy)), 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repo, policy), []byte(singleRootPolicyJSON), 0o644)).To(Succeed())
		gitIn(repo, "--literal-pathspecs", "add", "--", policy)
	}
	skipWorktreeMark := func(repo, policy string) string {
		return strings.TrimSpace(gitIn(repo, "--literal-pathspecs", "ls-files", "-v", "--", policy))
	}

	DescribeTable("the command that clears a skip-worktree mark",
		func(policy string) {
			repo := commitScannableGoRepo()
			commitFile(repo, policy, singleRootPolicyJSON)
			gitIn(repo, "--literal-pathspecs", "update-index", "--skip-worktree", "--", policy)
			deleteFromDisk(repo, policy)

			stderr := scanExplicitPolicy(repo, policy)
			command := printedClearMarkCommand(stderr)
			Expect(command).NotTo(BeEmpty(), "the mark stops a restore, so the message names how to clear it:\n%s", stderr)
			Expect(skipWorktreeMark(repo, policy)).To(HavePrefix("S "), "the policy starts out marked")
			subdirectory := filepath.Join(repo, "docs", "deeper")
			Expect(os.MkdirAll(subdirectory, 0o755)).To(Succeed())
			output, exitCode := runInShell(subdirectory, command)

			Expect(exitCode).To(Equal(0), "the printed command must work when pasted from a subdirectory:\ncommand: %s\noutput: %s\nstderr:\n%s", command, output, stderr)
			Expect(skipWorktreeMark(repo, policy)).To(HavePrefix("H "), "the command must clear the mark, not set it")
			restoreStderr := scanExplicitPolicy(repo, policy)
			restore := printedRestoreCommand(restoreStderr)
			Expect(restore).NotTo(BeEmpty(), "with the mark cleared, the policy is restorable:\n%s", restoreStderr)
			restoreOutput, restoreExitCode := runInShell(repo, restore)
			Expect(restoreExitCode).To(Equal(0), "command: %s\noutput: %s", restore, restoreOutput)
			Expect(filepath.Join(repo, policy)).To(BeARegularFile())
		},
		func(policy string) string { return "the policy " + policy },
		Entry(nil, policyNames[0]), Entry(nil, policyNames[1]), Entry(nil, policyNames[2]),
	)

	DescribeTable("the command that restores a staged copy nothing commits, pasted from a subdirectory of the repository",
		func(policy string) {
			repo := commitScannableGoRepo()
			stageWithoutCommitting(repo, policy)
			deleteFromDisk(repo, policy)
			subdirectory := filepath.Join(repo, "docs", "deeper")
			Expect(os.MkdirAll(subdirectory, 0o755)).To(Succeed())

			stderr := scanExplicitPolicy(repo, policy)
			command := printedRestoreCommand(stderr)
			Expect(command).NotTo(BeEmpty(), "a staged copy is restorable from the index:\n%s", stderr)
			Expect(command).NotTo(ContainSubstring("--source"), "nothing commits the policy, so it is restored from the index:\n%s", command)
			output, exitCode := runInShell(subdirectory, command)

			Expect(exitCode).To(Equal(0), "the printed command must work when pasted from any directory:\ncommand: %s\noutput: %s\nstderr:\n%s", command, output, stderr)
			onDisk, err := os.ReadFile(filepath.Join(repo, policy))
			Expect(err).NotTo(HaveOccurred(), "the staged copy is back where the policy was")
			Expect(string(onDisk)).To(Equal(singleRootPolicyJSON))
		},
		func(policy string) string { return "the policy " + policy },
		Entry(nil, policyNames[0]), Entry(nil, policyNames[1]), Entry(nil, policyNames[2]),
	)
})
