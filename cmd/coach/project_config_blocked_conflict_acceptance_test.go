package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	conflictedPolicy = "p/policy.json"
	theUsersFile     = "PRECIOUS USER DATA\n"
	moveStep         = "move the file that blocks the path, then "
)

var (
	printedCheckout = regexp.MustCompile(`git checkout --(merge|ours|theirs) -- ':\(top,literal\)p/policy\.json'`)
	printedRemoval  = regexp.MustCompile(`git rm -- ':\(top,literal\)p/policy\.json'`)
)

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

// Every command that brings the conflicted file back is a pathspec checkout, which git forces, and git unlinks the
// untracked file that stands where the policy's directory belongs to make the directory. The message already says
// to move that file; the command must not be offered ahead of the move.
func expectEveryCheckoutBehindTheMove(message string) {
	GinkgoHelper()
	found := printedCheckout.FindAllStringIndex(message, -1)
	for _, at := range found {
		before := message[max(0, at[0]-200):at[0]]
		Expect(before).To(MatchRegexp(regexp.QuoteMeta(moveStep)+`[^;]*$`), "a checkout offered before the file in the way is moved:\n%s", message)
	}
}

var _ = Describe("coach codesignal: advice for a conflicted policy does not delete the file in the way of its path", func() {
	DescribeTable("a scan names the move before any checkout, and the checkout works once the file is moved",
		func(kind string, wantsCheckout bool) {
			repo := aConflictedPolicyBehindAFile(kind)

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", conflictedPolicy, "--no-interactive")

			Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
			message := string(stderr)
			Expect(message).To(ContainSubstring("project_config_missing"))
			Expect(message).To(ContainSubstring("a parent path component is not a directory"))
			expectEveryCheckoutBehindTheMove(message)
			checkout := printedCheckout.FindString(message)
			Expect(checkout != "").To(Equal(wantsCheckout), "stderr:\n%s", message)

			if checkout != "" {
				moved := copyOf(repo)
				output, code := runInShell(moved, "mv p p.moved && "+checkout)
				Expect(code).To(Equal(0), "the checkout after the move: %s", output)
				Expect(os.ReadFile(filepath.Join(moved, "p", "policy.json"))).NotTo(BeEmpty())
				Expect(os.ReadFile(filepath.Join(moved, "p.moved"))).To(Equal([]byte(theUsersFile)), "the file in the way was moved, not lost")

				careless := copyOf(repo)
				output, code = runInShell(careless, checkout)
				Expect(code).To(Equal(0), "git forces the pathspec checkout: %s", output)
				Expect(filepath.Join(careless, "p.moved")).NotTo(BeAnExistingFile())
				Expect(os.ReadFile(filepath.Join(careless, "p", "policy.json"))).NotTo(BeEmpty(), "without the move, the checkout made the directory in place of the user's file: this is why the move comes first")
			}
			removal := printedRemoval.FindString(message)
			if removal != "" {
				unmoved := copyOf(repo)
				output, code := runInShell(unmoved, removal)
				Expect(code).To(Equal(0), "%s", output)
				Expect(os.ReadFile(filepath.Join(unmoved, "p"))).To(Equal([]byte(theUsersFile)), "accepting the deletion leaves the file in the way alone")
			}
		},
		Entry("both sides changed it", "UU", true),
		Entry("we changed it and they deleted it", "UD", true),
		Entry("we deleted it and they changed it", "DU", true),
	)

	It("is described by the README: the checkout is behind the step that moves the file", func() {
		Expect(readmeBullet("A path under a regular file")).To(ContainSubstring("is named behind the step that moves the file"))
	})

	It("--check-project prints no checkout for it", func() {
		repo := aConflictedPolicyBehindAFile("UU")

		stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, pathWithoutNode(), "--baseline", "--check-project", "--project-language", "typescript", "--project-config", conflictedPolicy)

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(string(stdout)).To(ContainSubstring("policy_missing cause=merge_conflict"))
		Expect(strings.Contains(string(stdout), "git checkout")).To(BeFalse(), "stdout:\n%s", stdout)
	})
})
