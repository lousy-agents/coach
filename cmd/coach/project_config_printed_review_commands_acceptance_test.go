package main

import (
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// printedReviewCommand is the git command a stderr names for the user to look at what the index holds
// (the staged edit, or the paths with an unresolved conflict), or "" when none is named.
var printedReviewCommand = func(stderr []byte) string {
	return regexp.MustCompile(`git (diff --cached|status)\b`).FindString(string(stderr))
}

// stageConflicting records an unresolved conflict for path in the index, one entry for each stage given, each with
// content of its own, so that git status lists it (identical sides collapse into a clean entry).
func stageConflicting(repo, path string, stages ...int) {
	gitIn(repo, "rm", "-q", "--cached", "--ignore-unmatch", "--", path)
	entries := ""
	for _, stage := range stages {
		blob := gitWithInput(repo, `{"schema_version":"1","roots":["side-`+string(rune('0'+stage))+`"]}`+"\n", "hash-object", "-w", "--stdin")
		entries += "100644 " + blob + " " + string(rune('0'+stage)) + "\t" + path + "\n"
	}
	gitWithInput(repo, entries, "update-index", "--index-info")
}

// The two review commands a message names for a policy whose index holds work (git diff --cached for a staged
// edit, git status for an unresolved merge conflict) are run as printed, because a command git rejects looks the
// same as a good one in the text, and each is expected to show what the message sends the user to.
var _ = Describe("coach codesignal: the review commands printed for a policy whose index holds work, run as printed", func() {
	const policy = "cfg/p.json"

	DescribeTable("shows what the message says to look at",
		func(arrange func(repo string), wantCommand, wantInOutput string) {
			repo := commitScannableGoRepo()
			arrange(repo)

			stderr := scanExplicitPolicy(repo, policy)
			command := printedReviewCommand(stderr)
			Expect(command).To(Equal(wantCommand), "the message names the command to review the index with:\n%s", stderr)
			output, exitCode := runInShell(repo, command)

			Expect(exitCode).To(Equal(0), "the printed command must work when pasted at the repository root:\ncommand: %s\noutput: %s", command, output)
			Expect(output).To(ContainSubstring(wantInOutput), "the command must show what the message sends the user to:\ncommand: %s\noutput: %s", command, output)
		},
		Entry("a staged edit to a committed policy", func(repo string) {
			commitFile(repo, policy, singleRootPolicyJSON)
			writeWorktreeFile(repo, policy, `{"schema_version":"1","roots":["staged"]}`+"\n")
			gitIn(repo, "add", policy)
			removeTree(repo, "cfg")
		}, "git diff --cached", `"roots":["staged"]`),
		Entry("an unresolved merge conflict over a committed policy", func(repo string) {
			commitFile(repo, policy, singleRootPolicyJSON)
			stageConflicting(repo, policy, 1, 2, 3)
			removeTree(repo, "cfg")
		}, "git status", "both modified"),
		Entry("an unresolved merge conflict for a policy nothing commits", func(repo string) {
			stageConflicting(repo, policy, 2, 3)
		}, "git status", "both added"),
	)
})
