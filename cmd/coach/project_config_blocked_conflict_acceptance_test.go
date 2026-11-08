package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	conflictedPolicy = "p/policy.json"
	theUsersFile     = "PRECIOUS USER DATA\n"
	moveStep         = "move the file that blocks the path, then "
)

// theStepsThatWriteThePath are the two ways the advice words the step that brings the conflicted file back: with the
// conflict markers git makes from both sides, or keeping the version of the one side that still has it.
var theStepsThatWriteThePath = []string{`bring back "p/policy.json" with the conflict markers`, `keep the version of "p/policy.json" that `}

// aConflictedPolicyBehindAFile merges a branch into one that changed the same policy so that its index entry is
// unmerged, and then leaves an untracked regular file where the policy's directory belongs. kind is how the sides
// stand: UU both changed it, UD we changed it and they deleted it, DU we deleted it and they changed it.
func aConflictedPolicyBehindAFile(kind string) string {
	GinkgoHelper()
	changed := func(root string) string { return `{"schema_version":"1","roots":["` + root + `"]}` + "\n" }
	repo := commitScannableGoRepo()
	commitFile(repo, conflictedPolicy, singleRootPolicyJSON)
	gitInWithIdentity(repo, "checkout", "-q", "-b", "other")
	if kind == "UD" {
		gitInWithIdentity(repo, "rm", "-q", conflictedPolicy)
		gitInWithIdentity(repo, "commit", "-q", "-m", "they delete it")
	} else {
		commitFile(repo, conflictedPolicy, changed("a"))
	}
	gitInWithIdentity(repo, "checkout", "-q", "-")
	if kind == "DU" {
		gitInWithIdentity(repo, "rm", "-q", conflictedPolicy)
		gitInWithIdentity(repo, "commit", "-q", "-m", "we delete it")
	} else {
		commitFile(repo, conflictedPolicy, changed("b"))
	}
	gitInWithIdentity(repo, "merge", "other")
	Expect(gitIn(repo, "status", "--porcelain")).To(ContainSubstring(kind+" "+conflictedPolicy), "the merge must leave the policy unmerged")
	Expect(os.RemoveAll(filepath.Join(repo, "p"))).To(Succeed())
	Expect(os.WriteFile(filepath.Join(repo, "p"), []byte(theUsersFile), 0o644)).To(Succeed())
	return repo
}

func copyOf(repo string) string {
	GinkgoHelper()
	destination := filepath.Join(GinkgoT().TempDir(), "copy")
	output, err := exec.Command("cp", "-R", repo, destination).CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "%s", output)
	return destination
}

// Bringing the conflicted file back is forced by git, and git unlinks the untracked file that stands where the policy's
// directory belongs to make the directory. The message says to move that file; the step that writes the path must not
// be described ahead of the move, and no command is printed for it.
func expectEveryStepBehindTheMove(message string) {
	GinkgoHelper()
	expectNoRunnableCommand(message)
	for _, step := range theStepsThatWriteThePath {
		for _, at := range offsetsOf(message, step) {
			before := message[max(0, at-len(moveStep)):at]
			Expect(before).To(Equal(moveStep), "the step %q is described before the file in the way is moved:\n%s", step, message)
		}
	}
}

// offsetsOf are the offsets of every occurrence of part in text.
func offsetsOf(text, part string) []int {
	var offsets []int
	for from := 0; from < len(text); {
		at := strings.Index(text[from:], part)
		if at < 0 {
			break
		}
		offsets = append(offsets, from+at)
		from += at + len(part)
	}
	return offsets
}

var _ = Describe("coach codesignal: advice for a conflicted policy does not delete the file in the way of its path", func() {
	DescribeTable("a scan names the move before the step that brings the file back, and the step works once the file is moved",
		func(kind, flag string, offersTheDeletion bool) {
			repo := aConflictedPolicyBehindAFile(kind)

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", conflictedPolicy, "--no-interactive")

			Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
			message := string(stderr)
			Expect(message).To(ContainSubstring("project_config_missing"))
			Expect(message).To(ContainSubstring("a parent path component is not a directory"))
			expectEveryStepBehindTheMove(message)
			Expect(message).To(ContainSubstring(moveStep), "the message says to move the file that blocks the path:\n%s", message)

			bringBack := []string{"checkout", flag, "--", ":(top,literal)" + conflictedPolicy}
			moved := copyOf(repo)
			Expect(os.Rename(filepath.Join(moved, "p"), filepath.Join(moved, "p.moved"))).To(Succeed(), "moving the file that blocks the path, as the message says")
			Expect(os.ReadFile(filepath.Join(moved, "p.moved"))).To(Equal([]byte(theUsersFile)), "the file in the way was moved, not lost")
			gitIn(moved, bringBack...)
			Expect(os.ReadFile(filepath.Join(moved, "p", "policy.json"))).NotTo(BeEmpty(), "bringing the file back after the move recreates it")

			careless := copyOf(repo)
			gitIn(careless, bringBack...)
			Expect(filepath.Join(careless, "p.moved")).NotTo(BeAnExistingFile())
			Expect(os.ReadFile(filepath.Join(careless, "p", "policy.json"))).NotTo(BeEmpty(), "without the move, the checkout made the directory in place of the user's file: this is why the move comes first")

			Expect(strings.Contains(message, `accept the deletion by recording "p/policy.json" as deleted and committing`)).To(Equal(offersTheDeletion), "stderr:\n%s", message)
		},
		Entry("both sides changed it", "UU", "--merge", false),
		Entry("we changed it and they deleted it", "UD", "--ours", true),
		Entry("we deleted it and they changed it", "DU", "--theirs", true),
	)

	It("accepts the deletion without moving the file in the way, as the message says", func() {
		repo := aConflictedPolicyBehindAFile("UD")

		gitIn(repo, "rm", "-q", "--", ":(top,literal)"+conflictedPolicy)

		Expect(os.ReadFile(filepath.Join(repo, "p"))).To(Equal([]byte(theUsersFile)), "accepting the deletion leaves the file in the way alone")
		Expect(gitIn(repo, "ls-files", "--unmerged")).To(BeEmpty())
	})

	It("is described by the README: the step is described behind the step that moves the file", func() {
		Expect(readmeBullet("A path under a regular file")).To(ContainSubstring("is described behind the step that moves the file"))
	})

	It("--check-project prints no command for it", func() {
		repo := aConflictedPolicyBehindAFile("UU")

		stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, pathWithoutNode(), "--baseline", "--check-project", "--project-language", "typescript", "--project-config", conflictedPolicy)

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(string(stdout)).To(ContainSubstring("policy_missing cause=merge_conflict"))
		expectNoRunnableCommand(string(stdout))
	})
})
