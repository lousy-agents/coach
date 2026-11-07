package main

import (
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func misspelledBase() (string, []string) {
	repo := newTempGitRepo()
	commitFile(repo, "a.go", "package a\n")
	return repo, []string{"codesignal", "--base", "nosuchref", "--no-interactive"}
}

func baseWithNoMergeBase() (string, []string) {
	repo := newTempGitRepo()
	commitFile(repo, "a.go", "package a\n")
	trunk := strings.TrimSpace(gitOutputIn(repo, "branch", "--show-current"))
	gitOutputIn(repo, "checkout", "--orphan", "other")
	gitOutputIn(repo, "rm", "-rf", "--quiet", ".")
	commitFile(repo, "b.go", "package b\n")
	gitOutputIn(repo, "checkout", "--quiet", trunk)
	return repo, []string{"codesignal", "--base", "other", "--no-interactive"}
}

func outsideARepository() (string, []string) {
	directory, err := os.MkdirTemp("", "coach-acceptance-no-repo-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, directory)
	return directory, []string{"codesignal", "--baseline", "--no-interactive"}
}

func anEmptyRepository() (string, []string) {
	return newTempGitRepo(), []string{"codesignal", "--baseline", "--no-interactive"}
}

// A git before 2.44 cannot be told not to fetch, and the guard that makes up for it asks which objects the scan
// would read are missing. That is a question about a revision, so a revision that does not resolve is not one it
// asks: the scan ends on it and says why, as it does on a current git. Upgrading git fixes none of these, and a
// message that sends the reader to do it hides the typo or the missing commit.
var _ = Describe("coach codesignal: on a git before 2.44 an ordinary failure is worded as on a current git", func() {
	DescribeTable("names its cause, and does not offer a newer git for it",
		func(scenario func() (string, []string), cause string) {
			directory, args := scenario()

			currentStdout, currentStderr, currentExit := runCoachBinary(commandPath, directory, os.Environ(), args...)
			oldStdout, oldStderr, oldExit := runCoachBinary(commandPath, directory, gitBefore244(), args...)

			Expect(string(currentStderr)).To(ContainSubstring(cause), "a current git must name the cause, or this proves nothing: %s", currentStderr)
			Expect(string(oldStderr)).To(Equal(string(currentStderr)), "on a git before 2.44")
			Expect(string(oldStdout)).To(Equal(string(currentStdout)))
			Expect(oldExit).To(Equal(currentExit))
			Expect(string(oldStderr)).NotTo(ContainSubstring("2.44"))
		},
		Entry("a misspelled --base", misspelledBase, `"nosuchref" cannot be resolved to a commit`),
		Entry("a --base with no merge base", baseWithNoMergeBase, `no merge base between "other" and HEAD`),
		Entry("a directory that is not a repository", outsideARepository, "is not inside a Git worktree"),
		Entry("a repository with no commits", anEmptyRepository, "HEAD is not readable"),
	)
})
