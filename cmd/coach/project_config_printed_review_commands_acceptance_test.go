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
	mergeOther(repo)
}

// mergeOther merges the branch "other" into the current one and lets it stop on a conflict. A merge needs a
// committer even when it stops on a conflict: with no identity configured (a container with no global git config)
// git exits 128 before it touches the files and no conflict exists.
func mergeOther(repo string) {
	merge := exec.Command("git", "-C", repo, "merge", "--no-edit", "other")
	merge.Env = commitEnv
	_ = merge.Run() // exits 1: the conflict is the point
}

// modifyDeleteConflictOver commits path, then has one branch delete it and the other change it, and merges them.
// Git stops with the path unmerged in the index: the base's version and the version of the side that kept the
// file, and none for the side that deleted it. There are not two versions for conflict markers to be made from.
func modifyDeleteConflictOver(repo, path string, headDeletes bool) {
	commitFile(repo, path, singleRootPolicyJSON)
	gitIn(repo, "checkout", "-q", "-b", "other")
	if headDeletes {
		commitFile(repo, path, `{"schema_version":"1","roots":["theirs"]}`+"\n")
	} else {
		removeFile(repo, path)
	}
	gitIn(repo, "checkout", "-q", "-")
	if headDeletes {
		removeFile(repo, path)
	} else {
		commitFile(repo, path, `{"schema_version":"1","roots":["ours"]}`+"\n")
	}
	mergeOther(repo)
}

// printedConflictCommands are the commands a message names for a file that one side of a merge deleted.
func printedConflictCommands(stderr []byte) []string {
	return regexp.MustCompile(`git (?:checkout --(?:ours|theirs)|rm) -- '[^']+'`).FindAllString(string(stderr), -1)
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

	// Reviewing the staged edit is half of it: the message also says how to keep it. git restore with no source
	// writes what the index holds, which is the staged content and not the committed copy, and it works from a
	// subdirectory because the pathspec is anchored at the top.
	It("keeps the staged edit, not the committed copy, when the printed restore from the index is run from a subdirectory", func() {
		repo := commitScannableGoRepo()
		commitFile(repo, policy, singleRootPolicyJSON)
		const staged = `{"schema_version":"1","roots":["staged"]}` + "\n"
		writeWorktreeFile(repo, policy, staged)
		gitIn(repo, "add", policy)
		removeTree(repo, "cfg")
		subdirectory := filepath.Join(repo, "docs", "deeper")
		Expect(os.MkdirAll(subdirectory, 0o755)).To(Succeed())

		stderr := scanExplicitPolicy(repo, policy)
		command := regexp.MustCompile(`git restore -- '[^']+'`).FindString(string(stderr))
		Expect(command).NotTo(BeEmpty(), "the message says how to keep the staged edit:\n%s", stderr)
		output, exitCode := runInShell(subdirectory, command)

		Expect(exitCode).To(Equal(0), "the printed command must work when pasted from a subdirectory:\ncommand: %s\noutput: %s", command, output)
		content, err := os.ReadFile(filepath.Join(repo, policy))
		Expect(err).NotTo(HaveOccurred(), "the command must write the file")
		Expect(string(content)).To(Equal(staged), "the staged edit comes back, and not the committed copy %q", singleRootPolicyJSON)
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

	// One side deleted the file and the other changed it: the index holds the base and the side that kept the file,
	// and git checkout --merge fails for want of the other. The message names what works, which is to keep the
	// version that is left, or to accept the deletion, and each is run as printed.
	type oneSideDeleted struct {
		arrange func(repo string)
		keep    string // the command that keeps the version that is left
		kept    string // what that version holds
	}
	const policyPathspec = "':(top,literal)cfg/p.json'"
	oneSideDeletedStates := func() []TableEntry {
		return []TableEntry{
			Entry("HEAD deleted a committed policy that the merged branch changed", oneSideDeleted{func(repo string) { modifyDeleteConflictOver(repo, policy, true) }, "git checkout --theirs -- " + policyPathspec, "theirs"}),
			Entry("the merged branch deleted a committed policy that HEAD changed", oneSideDeleted{func(repo string) { modifyDeleteConflictOver(repo, policy, false) }, "git checkout --ours -- " + policyPathspec, "ours"}),
			Entry("a policy nothing commits, kept by the merged branch", oneSideDeleted{func(repo string) { stageConflicting(repo, policy, 1, 3) }, "git checkout --theirs -- " + policyPathspec, "side-3"}),
			Entry("a policy nothing commits, kept by HEAD", oneSideDeleted{func(repo string) { stageConflicting(repo, policy, 1, 2) }, "git checkout --ours -- " + policyPathspec, "side-2"}),
			Entry("a policy nothing commits, added by the merged branch alone", oneSideDeleted{func(repo string) { stageConflicting(repo, policy, 3) }, "git checkout --theirs -- " + policyPathspec, "side-3"}),
			Entry("a policy nothing commits, added by HEAD alone", oneSideDeleted{func(repo string) { stageConflicting(repo, policy, 2) }, "git checkout --ours -- " + policyPathspec, "side-2"}),
		}
	}
	scanTheConflictedPolicy := func(arrange func(repo string)) (repo string, stderr []byte) {
		repo = commitScannableGoRepo()
		arrange(repo)
		removeTree(repo, "cfg")
		return repo, scanExplicitPolicy(repo, policy)
	}

	DescribeTable("names no checkout --merge, which fails, but the commands that work, for a file one side deleted",
		func(s oneSideDeleted) {
			_, stderr := scanTheConflictedPolicy(s.arrange)

			Expect(string(stderr)).NotTo(ContainSubstring("checkout --merge"), "git cannot make conflict markers from one version:\n%s", stderr)
			Expect(printedConflictCommands(stderr)).To(Equal([]string{s.keep, "git rm -- " + policyPathspec}), "the version that is left, or the deletion:\n%s", stderr)
		},
		oneSideDeletedStates(),
	)

	DescribeTable("keeps the version that is left when the printed command is run, and git add resolves the conflict",
		func(s oneSideDeleted) {
			repo, stderr := scanTheConflictedPolicy(s.arrange)
			commands := printedConflictCommands(stderr)
			Expect(commands).To(HaveLen(2), "the version that is left, and the deletion:\n%s", stderr)
			command := commands[0]

			output, exitCode := runInShell(repo, command)

			Expect(exitCode).To(Equal(0), "the printed command must work when pasted at the repository root:\ncommand: %s\noutput: %s", command, output)
			content, err := os.ReadFile(filepath.Join(repo, policy))
			Expect(err).NotTo(HaveOccurred(), "the command must recreate the file")
			Expect(string(content)).To(ContainSubstring(s.kept), "the version that is left is back:\n%s", content)
			gitIn(repo, "add", "--", ":(top,literal)"+policy)
			Expect(gitIn(repo, "ls-files", "--unmerged")).To(BeEmpty(), "git add resolves the conflict")
		},
		oneSideDeletedStates(),
	)

	DescribeTable("accepts the deletion when the printed command is run, and no conflict is left",
		func(s oneSideDeleted) {
			repo, stderr := scanTheConflictedPolicy(s.arrange)
			commands := printedConflictCommands(stderr)
			Expect(commands).To(HaveLen(2), "the version that is left, and the deletion:\n%s", stderr)
			command := commands[1]

			output, exitCode := runInShell(repo, command)

			Expect(exitCode).To(Equal(0), "the printed command must work when pasted at the repository root:\ncommand: %s\noutput: %s", command, output)
			Expect(gitIn(repo, "ls-files", "--unmerged")).To(BeEmpty(), "git rm resolves the conflict")
		},
		oneSideDeletedStates(),
	)

	It("names the deletion alone when both sides deleted the file", func() {
		_, stderr := scanTheConflictedPolicy(func(repo string) { stageConflicting(repo, policy, 1) })

		Expect(string(stderr)).NotTo(ContainSubstring("checkout"), "no side holds a version to keep:\n%s", stderr)
		Expect(printedConflictCommands(stderr)).To(Equal([]string{"git rm -- " + policyPathspec}), "stderr:\n%s", stderr)
	})
})
