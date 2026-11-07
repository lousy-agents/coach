package main

import (
	"os"
	"path/filepath"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	committedSpelling = ".lousy-agents/Coach.json"
	typedSpelling     = ".lousy-agents/coach.json"
	usersPolicy       = `{"schema_version":"1","roots":["MYWORK"]}` + "\n"
)

var printedMove = regexp.MustCompile(`\bmv -- `)

func skipUnlessTheFileSystemTellsCaseApart(directory string) {
	GinkgoHelper()
	probe := filepath.Join(directory, "case-probe")
	Expect(os.WriteFile(probe, nil, 0o644)).To(Succeed())
	DeferCleanup(os.Remove, probe)
	if _, err := os.Lstat(filepath.Join(directory, "CASE-PROBE")); err == nil {
		Skip("this file system ignores case, so a case-only rename is the same file; the spec is for a disk that tells cases apart (Linux, a case-sensitive volume)")
	}
}

// aRepositoryWhoseGitIgnoresCaseOnADiskThatDoesNot is what a repository cloned on macOS or Windows and then copied to
// a case-sensitive disk looks like: git still has core.ignorecase set, because it probes the disk only at init or at
// clone. The revision commits the policy as Coach.json, and the user has written a new, untracked coach.json.
func aRepositoryWhoseGitIgnoresCaseOnADiskThatDoesNot(extraSpellings ...string) string {
	GinkgoHelper()
	repo := commitScannableGoRepo()
	skipUnlessTheFileSystemTellsCaseApart(repo)
	commitFile(repo, committedSpelling, singleRootPolicyJSON)
	for _, spelling := range extraSpellings {
		commitFile(repo, spelling, singleRootPolicyJSON)
	}
	gitIn(repo, "config", "core.ignorecase", "true")
	Expect(os.WriteFile(filepath.Join(repo, typedSpelling), []byte(usersPolicy), 0o644)).To(Succeed())
	return repo
}

// With core.ignorecase set, git mv skips its check that the destination exists for a move that changes only the case
// of a name, and renames over it: the command Coach printed deleted the user's file, which was never in the object
// store. A move is printed only where nothing else stands at the destination, or where the destination is the same
// file seen through a disk that folds case.
var _ = Describe("coach codesignal: git set to ignore case on a disk that does not never prints a move over the user's file", func() {
	expectTheUsersFileIsSafe := func(repo, message string) {
		GinkgoHelper()
		Expect(message).NotTo(MatchRegexp(printedMove.String()), "no git mv may be printed over a different file:\n%s", message)
		Expect(message).NotTo(ContainSubstring("this file system ignores case"), "the disk tells cases apart; only git's setting says otherwise:\n%s", message)
		if command := regexp.MustCompile(`(top="\$\(git rev-parse --show-toplevel\)" && )?(mkdir -p [^&]*&& )?git -C [^ ]+ mv -- [^ ]+ [^ ]+`).FindString(message); command != "" {
			pasted := copyOf(repo)
			_, _ = runInShell(pasted, command)
			Expect(os.ReadFile(filepath.Join(pasted, typedSpelling))).To(Equal([]byte(usersPolicy)), "whatever was pasted, the untracked file survives")
		}
		Expect(os.ReadFile(filepath.Join(repo, typedSpelling))).To(Equal([]byte(usersPolicy)))
	}

	It("a named policy gets no git mv and says that a different file stands there", func() {
		repo := aRepositoryWhoseGitIgnoresCaseOnADiskThatDoesNot()

		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", typedSpelling, "--no-interactive")

		Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring("project_config_invalid"))
		Expect(string(stderr)).To(ContainSubstring("a different file stands at"), "stderr:\n%s", stderr)
		expectTheUsersFileIsSafe(repo, string(stderr))
	})

	It("a discovered policy gets no git mv and says that a different file stands there", func() {
		repo := aRepositoryWhoseGitIgnoresCaseOnADiskThatDoesNot()

		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--no-interactive")

		Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring("project_config_invalid"))
		Expect(string(stderr)).To(ContainSubstring("a different file stands at"), "stderr:\n%s", stderr)
		expectTheUsersFileIsSafe(repo, string(stderr))
	})

	It("a policy committed under several spellings gets no command that moves over the user's file", func() {
		repo := aRepositoryWhoseGitIgnoresCaseOnADiskThatDoesNot(".lousy-agents/COACH.json")

		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--no-interactive")

		Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring("several spellings"))
		expectTheUsersFileIsSafe(repo, string(stderr))
	})
})

var _ = Describe("docs: a git mv over a different file is described as withheld", func() {
	It("says so in the CLI reference", func() {
		Expect(documentText("docs/cli-codesignal.md")).To(ContainSubstring("a different file stands at the destination and prints no command"))
	})
})
