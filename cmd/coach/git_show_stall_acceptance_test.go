//go:build unix

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A --base scan reads each changed file from git on its own: the content of each side (git show) and the
// lines that changed (git diff). When git does not answer for one of them, it will not answer for the next
// one either, so the scan ends with the first and does not wait out the limit once per file: with three
// changed files, three limits would take longer than the ceiling.
var _ = Describe("coach codesignal --base: a git that does not answer when it is asked for a file", func() {
	const (
		changedFiles = 3
		readLimit    = 30 * time.Second
		// ceiling holds one limit, and not two: the second file is never asked for.
		ceiling = readLimit + 25*time.Second
	)

	// three Go files are committed and then each changed: --base HEAD~3 is the revision before the changes.
	threeChangedGoFiles := func() string {
		repo := newTempGitRepo()
		for _, name := range []string{"a.go", "b.go", "c.go"} {
			commitFile(repo, name, minimalGoSource)
		}
		for _, name := range []string{"a.go", "b.go", "c.go"} {
			commitFile(repo, name, minimalGoSource+"\n// changed\n")
		}
		return repo
	}
	// one Go file is removed after a second one: --base HEAD~1 is the revision that still has it.
	oneRemovedGoFile := func() string {
		repo := newTempGitRepo()
		commitFile(repo, "b.go", minimalGoSource)
		commitFile(repo, "a.go", minimalGoSource)
		removeFile(repo, "a.go")
		return repo
	}

	showOfAGoFile := func(pattern string) string {
		return `show) shows=1 ;; ` + pattern + `) if [ -n "$shows" ]; then exec sleep 299; fi ;;`
	}
	anyRevision := func(string) string { return showOfAGoFile("*:*.go") }
	theBaseRevision := func(repo string) string {
		return showOfAGoFile(strings.TrimSpace(gitIn(repo, "rev-parse", "HEAD~3")) + ":*.go")
	}
	theChangedLines := func(string) string { return `--unified=0) exec sleep 299 ;;` }

	DescribeTable("ends with the first file it could not read, with an operational error that names the git question",
		func(buildRepo func() string, ruleFor func(repo string) string, base, question string) {
			repo := buildRepo()
			env := stubToolchainEnv(gitThatNeverAnswers(ruleFor(repo)) + string(os.PathListSeparator) + pathWithStubNode("v24.9.9"))

			stderr, exitCode, elapsed := runCoachWithin(repo, env, changedFiles*readLimit+20*time.Second, "codesignal", "--base", base)

			Expect(elapsed).To(BeNumerically("<", ceiling), "the scan waited %s for git; stderr: %s", elapsed, stderr)
			Expect(exitCode).To(Equal(1), "an operational failure: stderr: %s", stderr)
			Expect(string(stderr)).To(MatchRegexp(question + `: .*timed out after 30s`))
		},
		Entry("the head content of a changed file", threeChangedGoFiles, anyRevision, "HEAD~3", "git show"),
		Entry("the base content of a changed file", threeChangedGoFiles, theBaseRevision, "HEAD~3", "git show"),
		Entry("the changed lines of a changed file", threeChangedGoFiles, theChangedLines, "HEAD~3", "git diff"),
		Entry("the base content of a removed file", oneRemovedGoFile, anyRevision, "HEAD~1", "git show"),
	)
})

// gitThatNeverAnswers is a git that answers every question, the reads of the policy included, but not the
// one the rule names. The rule is the body of a sh case over each argument; shows is set once git show has
// been seen. It returns the directory that holds the git, to put first on PATH.
func gitThatNeverAnswers(rule string) string {
	realGit, err := exec.LookPath("git")
	Expect(err).NotTo(HaveOccurred())
	dir, err := os.MkdirTemp("", "coach-acceptance-stalled-git-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)
	script := "#!/bin/sh\nshows=\nfor argument in \"$@\"; do\n  case \"$argument\" in\n    " + rule + "\n  esac\ndone\nexec " + realGit + " \"$@\"\n"
	Expect(os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755)).To(Succeed())
	return dir
}
