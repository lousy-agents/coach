package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// aGitThatKnowsNoGlobalOptionOfCoach is a whole git, and not a wrapper around the real one: a program that does what
// a git before 2.15 does with a global option it has not heard of. It refuses --no-optional-locks wherever it stands
// in the arguments, with git's own words and exit 129, and Coach passes that option to every call, the question of
// the version among them. A call without the option is one Coach never makes, so the program fails it loudly and the
// spec fails with it.
func aGitThatKnowsNoGlobalOptionOfCoach() string {
	GinkgoHelper()
	directory := GinkgoT().TempDir()
	script := "#!/bin/sh\n" +
		"for argument in \"$@\"; do\n" +
		"\tif [ \"$argument\" = --no-optional-locks ]; then\n" +
		"\t\techo 'unknown option: --no-optional-locks' >&2\n" +
		"\t\techo 'usage: git [--version] [--help] [-C <path>] [-c <name>=<value>]' >&2\n" +
		"\t\texit 129\n" +
		"\tfi\n" +
		"done\n" +
		"echo \"the git before 2.15 was asked something without --no-optional-locks: $*\" >&2\n" +
		"exit 99\n"
	Expect(os.WriteFile(filepath.Join(directory, "git"), []byte(script), 0o755)).To(Succeed())
	return directory
}

// The version is the first question Coach asks git, and a git that fails it cannot be read with the options Coach
// passes to every call. Saying that the directory is not a worktree sends the reader to look at the repository; the
// cause is the git.
var _ = Describe("coach codesignal: a git that cannot answer the question of its version is named, not mistaken for a missing repository", func() {
	DescribeTable("exits 1 and says what git said and which git Coach needs",
		func(args ...string) {
			repo := commitScannableGoRepo()
			env := []string{"PATH=" + aGitThatKnowsNoGlobalOptionOfCoach(), "HOME=" + os.Getenv("HOME")}

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, env, args...)

			Expect(exitCode).To(Equal(1), "stdout: %s\nstderr: %s", stdout, stderr)
			Expect(string(stdout)).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("could not determine the version of git"), "stderr:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring("unknown option: --no-optional-locks"), "git's own words:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring("git 2.16 or later"), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("is not inside a Git worktree"), "the repository is fine; the git is not:\n%s", stderr)
		},
		Entry("a baseline scan", "codesignal", "--baseline", "--format=json", "--no-interactive"),
		Entry("a diff scan", "codesignal", "--base", "HEAD", "--format=json", "--no-interactive"),
		Entry("--check-project", "codesignal", "--baseline", "--check-project", "--project-language", "typescript"),
	)
})

// The Git version table quotes what a git before 2.15 gets, so the quote is the output of the same program.
var _ = Describe("docs/cli-codesignal.md: the Git version table quotes the message of a git that does not know --no-optional-locks", func() {
	It("holds the words the program above produces", func() {
		table := documentText("docs/cli-codesignal.md")

		Expect(table).To(ContainSubstring(`could not determine the version of git (exit status 129: "unknown option: --no-optional-locks\nusage: git [--version] [--help] [-C <path>] [-c <name>=<value>]"); Coach needs git 2.16 or later`))
		Expect(table).NotTo(ContainSubstring("error: unknown option --no-optional-locks"), "that is not what a git says")
	})
})
