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

	// The two scans wait out the same 30 seconds, so they run at once. The scope is all so that a scan does not
	// read the whole tree for its source scope first; it is the read of the one file that this spec is about.
	It("ends a --base scan with the read of that file, naming git show, and a --baseline scan with the request for it, naming git cat-file", func() {
		baseRepo := newTempGitRepo()
		commitFile(baseRepo, "a.go", minimalGoSource)
		commitFile(baseRepo, "a.go", minimalGoSource+"\n// changed\n")
		blockObject(baseRepo, "a.go")
		baselineRepo := commitScannableGoRepo()
		blockObject(baselineRepo, "a.go")
		env := stubToolchainEnv(pathWithStubNode("v24.9.9"))
		scans := []pipeScan{
			{name: "--base: git show", repo: baseRepo, env: env, args: []string{"codesignal", "--base", "HEAD~1", "--scope", "all"}, wantExit: 1, question: `git show: .*timed out after 30s`},
			{name: "--baseline: git cat-file", repo: baselineRepo, env: env, args: []string{"codesignal", "--baseline", "--scope", "all"}, wantExit: 1, question: `git cat-file: .*timed out after 30s`},
		}

		results := runAtOnce(scans, 3*readLimit+20*time.Second)

		Expect(pipeProblems(scans, results, ceiling)).To(BeEmpty())
	})
})
