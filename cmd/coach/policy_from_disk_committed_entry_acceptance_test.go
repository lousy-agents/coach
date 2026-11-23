//go:build unix

package main

import (
	"os"
	"path/filepath"
	"syscall"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal reads the project policy from disk: a policy committed as an entry that is not a file, beside the same kind of entry on disk (#168 A2b)", func() {
	const customPolicyPath = "policies/custom.json"

	commitLinkAt := func(path string) (repo, base string) {
		repo, base = scopePolicyRepo(scopePolicyFile{"real.json", singleRootPolicyJSON})
		symlinkAt(repo, path)
		gitIn(repo, "add", path)
		gitInWithIdentity(repo, "commit", "-q", "-m", "the policy as a link")
		base = scopePolicyHead(repo)
		commitFile(repo, "c.go", "package a\n\nfunc C() {}\n")
		Expect(runGitInRepo(repo, "ls-tree", "HEAD", path)).To(HavePrefix("120000 "), "fixture: the committed policy is a symbolic link")
		Expect(os.Remove(filepath.Join(repo, path))).To(Succeed())
		Expect(os.Symlink("/etc/hostname", filepath.Join(repo, path))).To(Succeed())
		return repo, base
	}
	commitDirectoryAt := func(path string) (repo, base string) {
		repo, base = scopePolicyRepo(scopePolicyFile{path + "/inner.json", singleRootPolicyJSON})
		Expect(runGitInRepo(repo, "ls-tree", "HEAD", path)).To(HavePrefix("040000 "), "fixture: the committed policy is a directory")
		return repo, base
	}

	for _, mode := range scopePolicyModes {
		for _, path := range []string{coachConfigPath, customPolicyPath} {
			args := func(extra ...string) []string {
				if path == customPolicyPath {
					return append([]string{"--project-config", path}, extra...)
				}
				return extra
			}

			It("#168 A2b: "+path+" committed as a symbolic link and a symbolic link to another target on disk exits 2 with the committed words and the stable cause, under "+mode.name, func() {
				repo, base := commitLinkAt(path)

				stdout, stderr, exitCode := mode.run(repo, base, args("--format=json")...)

				expectDiskJudgedRefusal(stdout, stderr, exitCode, path, "committed as a symbolic link", "cause=symbolic_link")
			})

			It("#168 A2b: "+path+" committed as a directory and a directory on disk exits 2 with the committed words and the stable cause, under "+mode.name, func() {
				repo, base := commitDirectoryAt(path)

				stdout, stderr, exitCode := mode.run(repo, base, args("--format=json")...)

				expectDiskJudgedRefusal(stdout, stderr, exitCode, path, "committed as a directory", "cause=not_a_regular_file")
			})

			It("#168 A2b: "+path+" committed as a symbolic link and a named pipe on disk exits 2 for the pipe, and does not name a link that is not there, under "+mode.name, func() {
				repo, base := commitLinkAt(path)
				Expect(os.Remove(filepath.Join(repo, path))).To(Succeed())
				Expect(syscall.Mkfifo(filepath.Join(repo, path), 0o644)).To(Succeed())

				stdout, stderr, exitCode := mode.run(repo, base, args("--format=json")...)

				expectDiskJudgedRefusal(stdout, stderr, exitCode, path, "cannot be used on disk", "cause=not_a_regular_file")
				Expect(string(stderr)).NotTo(ContainSubstring(" at revision "), "the entry on disk is not the committed link")
				Expect(string(stderr)).NotTo(ContainSubstring("committed as a symbolic link"))
			})

			It("#168 A2b: "+path+" committed as a directory and a named pipe on disk exits 2 for the pipe, and does not name a directory that is not there, under "+mode.name, func() {
				repo, base := commitDirectoryAt(path)
				Expect(os.RemoveAll(filepath.Join(repo, path))).To(Succeed())
				Expect(syscall.Mkfifo(filepath.Join(repo, path), 0o644)).To(Succeed())

				stdout, stderr, exitCode := mode.run(repo, base, args("--format=json")...)

				expectDiskJudgedRefusal(stdout, stderr, exitCode, path, "cannot be used on disk", "cause=not_a_regular_file")
				Expect(string(stderr)).NotTo(ContainSubstring(" at revision "), "the entry on disk is not the committed directory")
				Expect(string(stderr)).NotTo(ContainSubstring("committed as a directory"))
			})
		}
	}
})
