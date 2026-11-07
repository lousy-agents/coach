//go:build unix

package main

import (
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A --baseline scan reads the content of every file it analyzes from one long-lived `git cat-file --batch
// -Z`. When that git stops answering, the scan ends with the request that was not answered, as an operational
// error that names the git question, in the time one read is held to; it does not wait for as long as git
// stays silent, and it does not go on to the next file.
var _ = Describe("coach codesignal --baseline: a git that does not answer the reader of file contents", func() {
	const (
		readLimit = 30 * time.Second
		ceiling   = readLimit + 25*time.Second
	)

	It("ends with an operational error that names git cat-file", func() {
		repo := commitScannableGoRepo()
		env := stubToolchainEnv(gitThatNeverAnswers(`-Z) exec sleep 299 ;;`) + string(os.PathListSeparator) + pathWithStubNode("v24.9.9"))

		stderr, exitCode, elapsed := runCoachWithin(repo, env, 3*readLimit+20*time.Second, "codesignal", "--baseline")

		Expect(elapsed).To(BeNumerically("<", ceiling), "the scan waited %s for git; stderr: %s", elapsed, stderr)
		Expect(exitCode).To(Equal(1), "an operational failure: stderr: %s", stderr)
		Expect(string(stderr)).To(MatchRegexp(`git cat-file: .*timed out after 30s`))
	})
})
