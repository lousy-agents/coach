package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// printedIndexCommand is the git command a stderr names for the user to deal with what the index holds (review a
// staged edit, or bring a conflicted file back), or "" when none is named.
func printedIndexCommand(stderr []byte) string {
	return regexp.MustCompile(`git (diff --cached|checkout --merge -- '[^']+')`).FindString(string(stderr))
}

// stageConflicting records an unresolved conflict for path in the index, one entry for each stage given, each with
// content of its own, so that git lists it as unmerged (identical sides collapse into a clean entry).
func stageConflicting(repo, path string, stages ...int) {
	gitIn(repo, "rm", "-q", "--cached", "--ignore-unmatch", "--", path)
	entries := ""
	for _, stage := range stages {
		blob := gitWithInput(repo, `{"schema_version":"1","roots":["side-`+string(rune('0'+stage))+`"]}`+"\n", "hash-object", "-w", "--stdin")
		entries += "100644 " + blob + " " + string(rune('0'+stage)) + "\t" + path + "\n"
	}
	gitWithInput(repo, entries, "update-index", "--index-info")
}

// mergeConflictOver commits path, edits it differently on two branches, and merges them: git stops with the file
// unmerged in the index and conflict markers in the working tree, as it does for a user.
func mergeConflictOver(repo, path string) {
	commitFile(repo, path, singleRootPolicyJSON)
	gitIn(repo, "checkout", "-q", "-b", "other")
	commitFile(repo, path, `{"schema_version":"1","roots":["theirs"]}`+"\n")
	gitIn(repo, "checkout", "-q", "-")
	commitFile(repo, path, `{"schema_version":"1","roots":["ours"]}`+"\n")
	// A merge needs a committer even when it stops on a conflict: with no identity configured (a container with no
	// global git config) git exits 128 before it touches the files and no conflict exists.
	merge := exec.Command("git", "-C", repo, "merge", "--no-edit", "other")
	merge.Env = commitEnv
	_ = merge.Run() // exits 1: the conflict is the point
}

// The commands a message names for a policy whose index holds work are run as printed, because a command git
// rejects looks the same as a good one in the text, and each is expected to do what the message says.
var _ = Describe("coach codesignal: the commands printed for a policy whose index holds work, run as printed", func() {
	const policy = "cfg/p.json"

	It("shows the staged edit to a committed policy that git diff --cached is named for", func() {
		repo := commitScannableGoRepo()
		commitFile(repo, policy, singleRootPolicyJSON)
		writeWorktreeFile(repo, policy, `{"schema_version":"1","roots":["staged"]}`+"\n")
		gitIn(repo, "add", policy)
		removeTree(repo, "cfg")

		stderr := scanExplicitPolicy(repo, policy)
		command := printedIndexCommand(stderr)
		Expect(command).To(Equal("git diff --cached"), "the message names the command to review the edit with:\n%s", stderr)
		output, exitCode := runInShell(repo, command)

		Expect(exitCode).To(Equal(0), "command: %s\noutput: %s", command, output)
		Expect(output).To(ContainSubstring(`"roots":["staged"]`), "the command must show the staged edit:\ncommand: %s\noutput: %s", command, output)
	})

	// The file is gone from disk (that is why the policy is missing), so a conflict cannot be resolved by editing
	// it. The command the message prints brings it back with its conflict markers, and both sides' content with
	// it; git add on the missing file would record a deletion and lose the other side.
	DescribeTable("brings the conflicted file back with both sides in it",
		func(arrange func(repo string), wantSides ...string) {
			repo := commitScannableGoRepo()
			arrange(repo)
			removeTree(repo, "cfg")

			stderr := scanExplicitPolicy(repo, policy)
			command := printedIndexCommand(stderr)
			Expect(command).To(HavePrefix("git checkout --merge -- "), "the message names the command that brings the file back:\n%s", stderr)
			output, exitCode := runInShell(repo, command)
			Expect(exitCode).To(Equal(0), "the printed command must work when pasted at the repository root:\ncommand: %s\noutput: %s", command, output)

			content, err := os.ReadFile(filepath.Join(repo, policy))
			Expect(err).NotTo(HaveOccurred(), "the command must recreate the file")
			Expect(string(content)).To(ContainSubstring("<<<<<<<"), "the conflict is back in the file:\n%s", content)
			for _, side := range wantSides {
				Expect(string(content)).To(ContainSubstring(side), "both sides' content survives:\n%s", content)
			}
		},
		Entry("a real merge conflict over a committed policy", func(repo string) { mergeConflictOver(repo, policy) }, "ours", "theirs"),
		Entry("a conflict for a policy nothing commits", func(repo string) { stageConflicting(repo, policy, 2, 3) }, "side-2", "side-3"),
	)
})
