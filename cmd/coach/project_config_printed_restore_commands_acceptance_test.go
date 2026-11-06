package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// printedRestoreCommand is the git restore command a stderr prints, to the end
// of its line, or "" when none is printed. It looks for any `git restore`, not
// for one wording of the offer, so a command printed under another sentence is
// still found.
func printedRestoreCommand(stderr []byte) string {
	for _, line := range strings.Split(string(stderr), "\n") {
		if index := strings.Index(line, "git restore"); index >= 0 {
			return strings.TrimSpace(line[index:])
		}
	}
	return ""
}

// The restore offers are the part of the missing-policy message a user pastes
// into a shell. Every state the index and the working tree can leave a policy in
// is therefore checked by doing what the message says: a printed command has to
// exit 0 and leave the policy on disk, and where it would not, none is printed.
var _ = Describe("coach codesignal: the restore command printed for a missing explicit policy", func() {
	const policy = "cfg/p.json"
	const staged = `{"schema_version":"1","roots":["staged"]}` + "\n"

	removeFromDisk := func(repo string) {
		Expect(os.RemoveAll(filepath.Join(repo, "cfg"))).To(Succeed())
	}
	// conflictOnPolicy leaves the policy unmerged: trunk and a side branch both
	// change it (committed on trunk first when committedOnTrunk), or trunk drops it
	// while the side changes it.
	conflictOnPolicy := func(repo string, committedOnTrunk bool) {
		commitFile(repo, policy, singleRootPolicyJSON)
		trunk := strings.TrimSpace(gitIn(repo, "rev-parse", "--abbrev-ref", "HEAD"))
		gitIn(repo, "checkout", "-q", "-b", "side")
		commitFile(repo, policy, staged)
		gitIn(repo, "checkout", "-q", trunk)
		if committedOnTrunk {
			commitFile(repo, policy, `{"schema_version":"1","roots":["ours"]}`+"\n")
		} else {
			gitIn(repo, "rm", "-q", policy)
			gitInWithIdentity(repo, "commit", "-q", "-m", "drop the policy")
		}
		gitInWithIdentity(repo, "merge", "side")
		Expect(gitIn(repo, "ls-files", "--stage", "--", policy)).To(ContainSubstring(" 1\t"), "the merge must leave the policy unmerged")
		removeFromDisk(repo)
	}
	stageOnly := func(repo string) {
		Expect(os.MkdirAll(filepath.Join(repo, "cfg"), 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repo, policy), []byte(staged), 0o644)).To(Succeed())
		gitIn(repo, "add", policy)
	}
	blockParentWithAFile := func(repo string) {
		removeFromDisk(repo)
		Expect(os.WriteFile(filepath.Join(repo, "cfg"), []byte("a file in the way"), 0o644)).To(Succeed())
	}

	DescribeTable("prints a command that works, or none",
		func(arrange func(repo string), wantsCommand bool, wantContent string) {
			repo := commitScannableGoRepo()
			arrange(repo)

			stderr := scanExplicitPolicy(repo, policy)
			command := printedRestoreCommand(stderr)

			if !wantsCommand {
				Expect(command).To(BeEmpty(), "a restore would not work here, so none may be printed:\n%s", stderr)
				return
			}
			Expect(command).NotTo(BeEmpty(), "this state is restorable, so the command is printed:\n%s", stderr)
			output, exitCode := runInShell(repo, command)
			Expect(exitCode).To(Equal(0), "the printed command must work when pasted:\ncommand: %s\noutput: %s\nstderr:\n%s", command, output, stderr)
			Expect(filepath.Join(repo, policy)).To(BeARegularFile())
			onDisk, err := os.ReadFile(filepath.Join(repo, policy))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(onDisk)).To(Equal(wantContent), "the restore brings back the policy it names")
		},

		// restorable
		Entry("committed, then deleted", func(repo string) {
			commitFile(repo, policy, singleRootPolicyJSON)
			removeFromDisk(repo)
		}, true, singleRootPolicyJSON),
		Entry("committed, edited on disk, then deleted", func(repo string) {
			commitFile(repo, policy, singleRootPolicyJSON)
			Expect(os.WriteFile(filepath.Join(repo, policy), []byte(staged), 0o644)).To(Succeed())
			removeFromDisk(repo)
		}, true, singleRootPolicyJSON),
		Entry("committed, then deleted with git rm", func(repo string) {
			commitFile(repo, policy, singleRootPolicyJSON)
			gitIn(repo, "rm", "-q", policy)
		}, true, singleRootPolicyJSON),
		Entry("committed, marked assume-unchanged, then deleted", func(repo string) {
			commitFile(repo, policy, singleRootPolicyJSON)
			gitIn(repo, "update-index", "--assume-unchanged", policy)
			removeFromDisk(repo)
		}, true, singleRootPolicyJSON),
		Entry("never committed, staged as a regular file, then deleted", func(repo string) {
			stageOnly(repo)
			removeFromDisk(repo)
		}, true, staged),

		// not restorable, because a restore would discard, recreate or fail
		Entry("committed with a staged edit, then deleted", func(repo string) {
			commitFile(repo, policy, singleRootPolicyJSON)
			Expect(os.WriteFile(filepath.Join(repo, policy), []byte(staged), 0o644)).To(Succeed())
			gitIn(repo, "add", policy)
			removeFromDisk(repo)
		}, false, ""),
		Entry("committed, marked skip-worktree, then deleted", func(repo string) {
			commitFile(repo, policy, singleRootPolicyJSON)
			gitIn(repo, "update-index", "--skip-worktree", policy)
			removeFromDisk(repo)
		}, false, ""),
		Entry("committed, left out by a sparse checkout, then deleted", func(repo string) {
			commitFile(repo, policy, singleRootPolicyJSON)
			gitIn(repo, "config", "core.sparseCheckout", "true")
			gitIn(repo, "update-index", "--skip-worktree", policy)
			removeFromDisk(repo)
		}, false, ""),
		Entry("committed, unmerged, then deleted", func(repo string) { conflictOnPolicy(repo, true) }, false, ""),
		Entry("committed as a symbolic link, then the link removed", func(repo string) {
			Expect(os.MkdirAll(filepath.Join(repo, "cfg"), 0o755)).To(Succeed())
			Expect(os.Symlink("../real.json", filepath.Join(repo, policy))).To(Succeed())
			gitIn(repo, "add", policy)
			gitInWithIdentity(repo, "commit", "-q", "-m", "policy as a link")
			removeFromDisk(repo)
		}, false, ""),
		Entry("committed, with a file where its directory should be", func(repo string) {
			commitFile(repo, policy, singleRootPolicyJSON)
			blockParentWithAFile(repo)
		}, false, ""),
		Entry("never committed, staged, marked skip-worktree, then deleted", func(repo string) {
			stageOnly(repo)
			gitIn(repo, "update-index", "--skip-worktree", policy)
			removeFromDisk(repo)
		}, false, ""),
		Entry("never committed, staged, left out by a sparse checkout, then deleted", func(repo string) {
			stageOnly(repo)
			gitIn(repo, "config", "core.sparseCheckout", "true")
			gitIn(repo, "update-index", "--skip-worktree", policy)
			removeFromDisk(repo)
		}, false, ""),
		Entry("never committed, unmerged, then deleted", func(repo string) { conflictOnPolicy(repo, false) }, false, ""),
		Entry("never committed, staged as a symbolic link, then the link removed", func(repo string) {
			Expect(os.MkdirAll(filepath.Join(repo, "cfg"), 0o755)).To(Succeed())
			Expect(os.Symlink("../real.json", filepath.Join(repo, policy))).To(Succeed())
			gitIn(repo, "add", policy)
			Expect(os.Remove(filepath.Join(repo, policy))).To(Succeed())
		}, false, ""),
		Entry("never committed, staged as a submodule entry", func(repo string) {
			head := strings.TrimSpace(gitIn(repo, "rev-parse", "HEAD"))
			gitIn(repo, "update-index", "--add", "--cacheinfo", "160000,"+head+","+policy)
		}, false, ""),
		Entry("never committed, staged, with a file where its directory should be", func(repo string) {
			stageOnly(repo)
			blockParentWithAFile(repo)
		}, false, ""),
		Entry("never committed, added with intent-to-add, then deleted", func(repo string) {
			Expect(os.MkdirAll(filepath.Join(repo, "cfg"), 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(repo, policy), []byte(staged), 0o644)).To(Succeed())
			gitIn(repo, "add", "-N", policy)
			removeFromDisk(repo)
		}, false, ""),
		Entry("never committed and not in the index", func(repo string) {}, false, ""),
	)

	// The skip-worktree mark alone stops a restore. An empty entry carrying it is
	// not an edit to review, since git diff --cached does not show it, so the user is
	// told what the mark needs, not sent to a diff that shows nothing.
	It("names the skip-worktree mark and not a staged edit when the empty intent-to-add entry of a committed policy is marked skip-worktree", func() {
		repo := commitScannableGoRepo()
		commitFile(repo, policy, singleRootPolicyJSON)
		gitIn(repo, "rm", "-q", "--cached", policy)
		gitIn(repo, "add", "-N", policy)
		gitIn(repo, "update-index", "--skip-worktree", policy)
		removeFromDisk(repo)

		stderr := scanExplicitPolicy(repo, policy)

		Expect(printedRestoreCommand(stderr)).To(BeEmpty(), "the mark stops a restore:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring("skip-worktree"), "stderr:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring("git update-index --no-skip-worktree"), "stderr:\n%s", stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("staged edit"), "an empty entry holds no edit:\n%s", stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("git diff --cached"), "git diff --cached does not show an intent-to-add entry:\n%s", stderr)
	})

	// git add --intent-to-add stages the empty blob and nothing else. Over a
	// committed policy it leaves no staged work to lose, so the committed copy is
	// restored, and the message says that instead of calling the empty entry an edit.
	It("restores the committed copy and says so when a committed policy was removed from the index, re-added with intent-to-add, and deleted", func() {
		repo := commitScannableGoRepo()
		commitFile(repo, policy, singleRootPolicyJSON)
		gitIn(repo, "rm", "-q", "--cached", policy)
		gitIn(repo, "add", "-N", policy)
		removeFromDisk(repo)

		stderr := scanExplicitPolicy(repo, policy)
		command := printedRestoreCommand(stderr)
		doc := expectCheckProject(repo, "--project-config", policy)

		Expect(string(stderr)).To(ContainSubstring("restoring brings back the committed copy"), "stderr:\n%s", stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("staged edit"), "an empty entry holds no edit:\n%s", stderr)
		Expect(command).To(HavePrefix("git restore --source="), "the restore comes from the analyzed revision:\n%s", stderr)
		action, named := nextActionOfKind(doc, "restore_policy")
		Expect(named).To(BeTrue(), "--check-project names the restore: %+v", doc.NextActions)
		Expect(action.Executable).To(BeFalse())
		output, exitCode := runInShell(repo, command)
		Expect(exitCode).To(Equal(0), "the printed command must work when pasted:\ncommand: %s\noutput: %s", command, output)
		onDisk, err := os.ReadFile(filepath.Join(repo, policy))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(onDisk)).To(Equal(singleRootPolicyJSON))
	})
})
