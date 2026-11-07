package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// plumb runs git in repo with input on its standard input and returns what it prints, trimmed.
func plumb(repo, input string, args ...string) string {
	command := exec.Command("git", args...)
	command.Dir = repo
	command.Env = commitEnv
	command.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	ExpectWithOffset(1, command.Run()).To(Succeed(), "git %s: %s", strings.Join(args, " "), stderr.String())
	return strings.TrimSpace(stdout.String())
}

// commitTreeWithGitDirectory commits, over the current commit, a tree that holds a .git directory named
// gitName whose config runs a program for git's file system monitor. The plumbing writes it as asked; git add
// would refuse it, which is how an ordinary checkout of such a tree is refused too.
func commitTreeWithGitDirectory(repo, gitName, marker string) {
	config := plumb(repo, fmt.Sprintf("[core]\n\trepositoryformatversion = 0\n\tbare = false\n\tfsmonitor = \"touch %s; false\"\n", marker), "hash-object", "-w", "--stdin")
	head := plumb(repo, "ref: refs/heads/main\n", "hash-object", "-w", "--stdin")
	keep := plumb(repo, "", "hash-object", "-w", "--stdin")
	keepTree := plumb(repo, fmt.Sprintf("100644 blob %s\t.keep\n", keep), "mktree")
	gitTree := plumb(repo, fmt.Sprintf("100644 blob %s\tHEAD\n100644 blob %s\tconfig\n040000 tree %s\tobjects\n040000 tree %s\trefs\n", head, config, keepTree, keepTree), "mktree")
	mod := plumb(repo, "module example.com/planted\n\ngo 1.22\n", "hash-object", "-w", "--stdin")
	source := plumb(repo, "package main\n\nfunc main() {}\n", "hash-object", "-w", "--stdin")
	root := plumb(repo, fmt.Sprintf("040000 tree %s\t%s\n100644 blob %s\tgo.mod\n100644 blob %s\tmain.go\n", gitTree, gitName, mod, source), "mktree")
	commit := plumb(repo, "", "commit-tree", root, "-m", "a tree with a git directory in it")
	plumb(repo, "", "update-ref", "HEAD", commit)
}

// cloneWhoseCheckoutFails clones repo. Git writes the objects and points HEAD at the commit, then refuses to
// check out the tree, so the clone is a repository whose HEAD is the planted tree and whose worktree is empty.
func cloneWhoseCheckoutFails(repo string) string {
	directory := filepath.Join(GinkgoT().TempDir(), "victim")
	clone := exec.Command("git", "clone", "-q", "file://"+repo, directory)
	clone.Env = commitEnv
	_ = clone.Run()
	_, err := os.Stat(filepath.Join(directory, ".git", "HEAD"))
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "the clone must exist for the spec to mean anything")
	return directory
}

// A tree that holds a .git directory is one git itself refuses to check out, and v0.11.0 refused to read it
// (git archive checks every path). The source snapshot is a directory Coach runs go list in, and Go's version
// control stamping runs git there, which runs the program a .git/config names. So the committed bytes must
// never reach the snapshot as a git directory, in any spelling a file system or git folds to one.
var _ = Describe("coach codesignal --build-target: a revision that commits a .git directory", func() {
	spellings := []string{".git", ".GIT", ".Git", "git~1", ".git ", ".git.", ".g\u200cit", ".git\\config", ".git:stream", ".GI\u200cT", ".\u206agit"}

	DescribeTable("is refused before go runs, and the committed config never runs",
		func(spelling string) {
			repo := commitScannableGoRepo()
			marker := filepath.Join(GinkgoT().TempDir(), "RAN")
			commitTreeWithGitDirectory(repo, spelling, marker)

			_, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--build-target", "./...", "--no-interactive")

			Expect(marker).NotTo(BeAnExistingFile(), "the program a committed .git/config names ran")
			Expect(exitCode).To(Equal(1), "stderr: %s", stderr)
			Expect(string(stderr)).To(ContainSubstring("git directory"), "stderr: %s", stderr)
		},
		Entry("the plain name", spellings[0]),
		Entry("in capitals", spellings[1]),
		Entry("in mixed case", spellings[2]),
		Entry("as NTFS names its short form", spellings[3]),
		Entry("with a trailing space", spellings[4]),
		Entry("with a trailing dot", spellings[5]),
		Entry("with a code point HFS+ ignores", spellings[6]),
		Entry("with a backslash after it", spellings[7]),
		Entry("with an NTFS stream after it", spellings[8]),
		Entry("in capitals around a code point HFS+ ignores", spellings[9]),
		Entry("with the lowest code point of the last range HFS+ ignores", spellings[10]),
	)

	It("is refused in a clone whose checkout git refused", func() {
		source := commitScannableGoRepo()
		marker := filepath.Join(GinkgoT().TempDir(), "RAN")
		commitTreeWithGitDirectory(source, ".git", marker)
		victim := cloneWhoseCheckoutFails(source)

		_, stderr, exitCode := runCoachCodesignalBaselineRaw(victim, "--build-target", "./...", "--no-interactive")

		Expect(marker).NotTo(BeAnExistingFile(), "the program a committed .git/config names ran")
		Expect(exitCode).To(Equal(1), "stderr: %s", stderr)
	})
})
