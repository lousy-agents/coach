//go:build unix

package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A named pipe where git expects a loose object makes the read of that object wait for as long as nobody
// writes to it. A scan reads the content of the files it analyzes from git on its own, and when git does not
// answer for one, the scan ends with that read, as an operational error that names the git question, in the
// time one read is held to. It is real git on a real object that does not answer, not a stand-in for git.

// blockObject replaces the loose object of path's content at HEAD by a named pipe, by a rename, so nothing is
// deleted, and puts it back when the spec ends.
func blockObject(repo, path string) {
	oid := strings.TrimSpace(gitIn(repo, "rev-parse", "HEAD:"+path))
	object := filepath.Join(repo, ".git", "objects", oid[:2], oid[2:])
	Expect(os.Rename(object, object+".orig")).To(Succeed(), "the object must be loose: a fresh commit writes it so")
	Expect(syscall.Mkfifo(object, 0o644)).To(Succeed())
	DeferCleanup(func() { _ = os.Remove(object); _ = os.Rename(object+".orig", object) })
}

var _ = Describe("coach codesignal: a named pipe where git expects the object of a file", func() {
	const (
		readLimit = 30 * time.Second
		ceiling   = readLimit + 25*time.Second
	)

	// the scope is all so that the scan does not read the whole tree for its source scope first; it is the
	// read of the one file that this spec is about
	It("ends a --base scan with the read of that file, naming git show", func() {
		repo := newTempGitRepo()
		commitFile(repo, "a.go", minimalGoSource)
		commitFile(repo, "a.go", minimalGoSource+"\n// changed\n")
		blockObject(repo, "a.go")
		env := stubToolchainEnv(pathWithStubNode("v24.9.9"))

		stderr, exitCode, elapsed := runCoachWithin(repo, env, 3*readLimit+20*time.Second, "codesignal", "--base", "HEAD~1", "--scope", "all")

		Expect(elapsed).To(BeNumerically("<", ceiling), "the scan waited %s for git; stderr: %s", elapsed, stderr)
		Expect(exitCode).To(Equal(1), "an operational failure: stderr: %s", stderr)
		Expect(string(stderr)).To(MatchRegexp(`git show: .*timed out after 30s`))
	})

	It("ends a --baseline scan with the request for that file, naming git cat-file", func() {
		repo := commitScannableGoRepo()
		blockObject(repo, "a.go")
		env := stubToolchainEnv(pathWithStubNode("v24.9.9"))

		stderr, exitCode, elapsed := runCoachWithin(repo, env, 3*readLimit+20*time.Second, "codesignal", "--baseline", "--scope", "all")

		Expect(elapsed).To(BeNumerically("<", ceiling), "the scan waited %s for git; stderr: %s", elapsed, stderr)
		Expect(exitCode).To(Equal(1), "an operational failure: stderr: %s", stderr)
		Expect(string(stderr)).To(MatchRegexp(`git cat-file: .*timed out after 30s`))
	})
})
