package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// printedIndexCommand is the git command a stderr names for the user to review what the index holds (a staged edit),
// or "" when none is named. A merge conflict is told in words and names no command.
func printedIndexCommand(stderr []byte) string {
	return regexp.MustCompile(`git diff --cached`).FindString(string(stderr))
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

	// Next to the review, the message names the way back to the committed copy: git restore from the revision writes
	// it to the index and the file, so the staged edit is dropped. It is pasted from a subdirectory too.
	It("brings back the committed copy and drops the staged edit when the printed restore from the revision is run from a subdirectory", func() {
		repo := commitScannableGoRepo()
		commitFile(repo, policy, singleRootPolicyJSON)
		writeWorktreeFile(repo, policy, `{"schema_version":"1","roots":["staged"]}`+"\n")
		gitIn(repo, "add", policy)
		removeTree(repo, "cfg")
		subdirectory := filepath.Join(repo, "docs", "deeper")
		Expect(os.MkdirAll(subdirectory, 0o755)).To(Succeed())

		stderr := scanExplicitPolicy(repo, policy)
		command := regexp.MustCompile(`git restore --source=\S+ --staged --worktree -- '[^']+'`).FindString(string(stderr))
		Expect(command).NotTo(BeEmpty(), "the message says how to drop the staged edit:\n%s", stderr)
		output, exitCode := runInShell(subdirectory, command)

		Expect(exitCode).To(Equal(0), "the printed command must work when pasted from a subdirectory:\ncommand: %s\noutput: %s", command, output)
		content, err := os.ReadFile(filepath.Join(repo, policy))
		Expect(err).NotTo(HaveOccurred(), "the command must write the file")
		Expect(string(content)).To(Equal(singleRootPolicyJSON), "the committed copy comes back, and not the staged edit")
		Expect(gitIn(repo, "diff", "--cached", "--name-only")).To(BeEmpty(), "the index holds the committed copy again")
	})

	// The file is gone from disk (that is why the policy is missing), so a conflict cannot be resolved by editing
	// it. The message says in words to bring it back with its conflict markers, and both sides' content with it;
	// marking the missing file resolved would record a deletion and lose the other side. It names no command, so
	// the step is carried out here as the words describe it, and it is expected to do what the message says.
	DescribeTable("says to bring the conflicted file back with both sides in it, and doing so does",
		func(arrange func(repo string), wantSides ...string) {
			repo := commitScannableGoRepo()
			arrange(repo)
			removeTree(repo, "cfg")

			stderr := scanExplicitPolicy(repo, policy)
			expectNoRunnableCommand(string(stderr))
			Expect(string(stderr)).To(ContainSubstring(`bring back "cfg/p.json" with the conflict markers git makes from the two sides, resolve the conflict in it, mark it resolved, and commit`), "stderr:\n%s", stderr)
			gitIn(repo, "checkout", "--merge", "--", ":(top,literal)"+policy)

			content, err := os.ReadFile(filepath.Join(repo, policy))
			Expect(err).NotTo(HaveOccurred(), "bringing the file back must recreate it")
			Expect(string(content)).To(ContainSubstring("<<<<<<<"), "the conflict is back in the file:\n%s", content)
			for _, side := range wantSides {
				Expect(string(content)).To(ContainSubstring(side), "both sides' content survives:\n%s", content)
			}
		},
		Entry("a real merge conflict over a committed policy", func(repo string) { mergeConflictOver(repo, policy) }, "ours", "theirs"),
		Entry("a conflict for a policy nothing commits", func(repo string) { stageConflicting(repo, policy, 2, 3) }, "side-2", "side-3"),
	)

	// One side deleted the file and the other changed it: the index holds the base and the side that kept the file,
	// and there are not two versions to make conflict markers from. The message says what works, which is to keep the
	// version that is left, or to accept the deletion, and names the side that has it. Each step is carried out here
	// as the words describe it.
	type oneSideDeleted struct {
		arrange func(repo string)
		side    string // the side the message names as holding the version, and the flag that brings it back
		flag    string
		kept    string // what that version holds
	}
	const theKept = `keep the version of "cfg/p.json" that `
	const theDeletion = `accept the deletion by recording "cfg/p.json" as deleted and committing`
	oneSideDeletedStates := func() []TableEntry {
		merged, into := "the side being merged", "the side being merged into"
		return []TableEntry{
			Entry("HEAD deleted a committed policy that the merged branch changed", oneSideDeleted{func(repo string) { modifyDeleteConflictOver(repo, policy, true) }, merged, "--theirs", "theirs"}),
			Entry("the merged branch deleted a committed policy that HEAD changed", oneSideDeleted{func(repo string) { modifyDeleteConflictOver(repo, policy, false) }, into, "--ours", "ours"}),
			Entry("a policy nothing commits, kept by the merged branch", oneSideDeleted{func(repo string) { stageConflicting(repo, policy, 1, 3) }, merged, "--theirs", "side-3"}),
			Entry("a policy nothing commits, kept by HEAD", oneSideDeleted{func(repo string) { stageConflicting(repo, policy, 1, 2) }, into, "--ours", "side-2"}),
			Entry("a policy nothing commits, added by the merged branch alone", oneSideDeleted{func(repo string) { stageConflicting(repo, policy, 3) }, merged, "--theirs", "side-3"}),
			Entry("a policy nothing commits, added by HEAD alone", oneSideDeleted{func(repo string) { stageConflicting(repo, policy, 2) }, into, "--ours", "side-2"}),
		}
	}
	scanTheConflictedPolicy := func(arrange func(repo string)) (repo string, stderr []byte) {
		repo = commitScannableGoRepo()
		arrange(repo)
		removeTree(repo, "cfg")
		return repo, scanExplicitPolicy(repo, policy)
	}

	DescribeTable("names the side that has the file and the deletion, and no conflict markers, for a file one side deleted",
		func(s oneSideDeleted) {
			_, stderr := scanTheConflictedPolicy(s.arrange)

			expectNoRunnableCommand(string(stderr))
			Expect(string(stderr)).To(ContainSubstring(theKept+s.side+" holds by bringing it back, mark it resolved, and commit, or "+theDeletion), "the version that is left, or the deletion:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("with the conflict markers"), "git cannot make conflict markers from one version:\n%s", stderr)
		},
		oneSideDeletedStates(),
	)

	DescribeTable("keeps the version that is left when the step is carried out as described, and marking it resolved resolves the conflict",
		func(s oneSideDeleted) {
			repo, _ := scanTheConflictedPolicy(s.arrange)

			gitIn(repo, "checkout", s.flag, "--", ":(top,literal)"+policy)

			content, err := os.ReadFile(filepath.Join(repo, policy))
			Expect(err).NotTo(HaveOccurred(), "bringing the version back must recreate the file")
			Expect(string(content)).To(ContainSubstring(s.kept), "the version that is left is back:\n%s", content)
			gitIn(repo, "add", "--", ":(top,literal)"+policy)
			Expect(gitIn(repo, "ls-files", "--unmerged")).To(BeEmpty(), "marking the file resolved resolves the conflict")
		},
		oneSideDeletedStates(),
	)

	DescribeTable("accepts the deletion when it is recorded as described, and no conflict is left",
		func(s oneSideDeleted) {
			repo, _ := scanTheConflictedPolicy(s.arrange)

			gitIn(repo, "rm", "-q", "--", ":(top,literal)"+policy)

			Expect(gitIn(repo, "ls-files", "--unmerged")).To(BeEmpty(), "recording the deletion resolves the conflict")
		},
		oneSideDeletedStates(),
	)

	It("names the deletion alone when both sides deleted the file", func() {
		_, stderr := scanTheConflictedPolicy(func(repo string) { stageConflicting(repo, policy, 1) })

		expectNoRunnableCommand(string(stderr))
		Expect(string(stderr)).To(ContainSubstring("both sides deleted it; "+theDeletion), "no side holds a version to keep:\n%s", stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("keep the version"), "no side holds a version to keep:\n%s", stderr)
	})
})
