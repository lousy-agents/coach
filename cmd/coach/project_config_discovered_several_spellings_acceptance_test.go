package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// commitSpellingsOfThePolicy commits, over the repository's commit, a tree that holds the policy under the two
// spellings: a directory as Coach reads it with a file that differs, and a directory that differs with the file as
// Coach reads it. The plumbing writes both, which no file system that folds case could check out together.
func commitSpellingsOfThePolicy(repo string) {
	policy := plumb(repo, singleRootPolicyJSON, "hash-object", "-w", "--stdin")
	file := plumb(repo, minimalGoSource, "hash-object", "-w", "--stdin")
	exactDirectory := plumb(repo, fmt.Sprintf("100644 blob %s\tCoach.json\n", policy), "mktree")
	otherDirectory := plumb(repo, fmt.Sprintf("100644 blob %s\tcoach.json\n", policy), "mktree")
	root := plumb(repo, fmt.Sprintf("100644 blob %s\ta.go\n040000 tree %s\t.lousy-agents\n040000 tree %s\t.LOUSY-AGENTS\n", file, exactDirectory, otherDirectory), "mktree")
	plumb(repo, "", "update-ref", "HEAD", plumb(repo, "", "commit-tree", root, "-m", "the policy under two spellings"))
}

func skipUnlessFilesystemTellsCasesApart() {
	probeDir := GinkgoT().TempDir()
	Expect(os.WriteFile(filepath.Join(probeDir, "case-probe"), nil, 0o644)).To(Succeed())
	if _, err := os.Lstat(filepath.Join(probeDir, "CASE-PROBE")); err == nil {
		Skip("this file system folds case, so a tree with two spellings of one path cannot be checked out")
	}
}

// A policy committed under another case is not read, and the refusal says where it is. With the policy committed
// under two spellings, naming one left the other committed and ignored, so the user fixed one and the scan
// still found a spelling it did not read.
var _ = Describe("coach codesignal: a policy committed under several spellings", func() {
	var repo string

	BeforeEach(func() {
		repo = commitScannableGoRepo()
		commitSpellingsOfThePolicy(repo)
	})

	It("is refused with every spelling named and the commands that keep one and drop the rest", func() {
		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--no-interactive")

		Expect(exitCode).To(Equal(2), "stdout: %s\nstderr: %s", stdout, stderr)
		Expect(string(stdout)).To(BeEmpty(), "no report beside a refusal")
		Expect(string(stderr)).To(ContainSubstring(`several spellings, ".LOUSY-AGENTS/coach.json", ".lousy-agents/Coach.json"`), "stderr:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring(`git -C "$top" mv -- '.lousy-agents/Coach.json' '.lousy-agents/coach.json' && git -C "$top" --literal-pathspecs rm --cached -q -- '.LOUSY-AGENTS/coach.json'`), "stderr:\n%s", stderr)
	})

	It("is read once the printed commands have been run and the result committed", func() {
		skipUnlessFilesystemTellsCasesApart()
		gitInWithIdentity(repo, "reset", "-q", "--hard", "HEAD")
		_, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json", "--no-interactive")
		Expect(exitCode).To(Equal(2), "stderr: %s", stderr)
		command := regexp.MustCompile(`to keep "[^"]+", run (.+?) and commit \(project_config_invalid\)`).FindStringSubmatch(string(stderr))
		Expect(command).NotTo(BeNil(), "the message prints the commands:\n%s", stderr)

		run := exec.Command("sh", "-c", command[1])
		run.Dir = repo
		output, err := run.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "the printed commands must work: %s", output)
		gitInWithIdentity(repo, "commit", "-q", "-m", "keep one spelling of the policy")

		stdout, rescanStderr, rescanExit := runCoachCodesignalBaselineRaw(repo, "--format=json", "--no-interactive")
		Expect(rescanExit).To(Equal(0), "stdout: %s\nstderr: %s", stdout, rescanStderr)
		Expect(string(stdout)).To(ContainSubstring(`"schema_version":"2"`), "the policy is now read, which turns the report into schema 2")
	})
})
