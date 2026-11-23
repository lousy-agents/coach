//go:build unix

package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal reads the project policy from disk: a discovered policy path that nothing commits and the disk holds an entry that is not a file (#168 A2b)", func() {
	type entryOnDisk struct {
		description string
		reason      string
		cause       string
		make        func(repo string)
	}
	entries := []entryOnDisk{
		{"a directory", "not a regular file", "cause=not_a_regular_file", func(repo string) {
			Expect(os.MkdirAll(filepath.Join(repo, coachConfigPath), 0o755)).To(Succeed())
		}},
		{"a symbolic link", "a symbolic link", "cause=symbolic_link", func(repo string) {
			Expect(os.MkdirAll(filepath.Join(repo, filepath.Dir(coachConfigPath)), 0o755)).To(Succeed())
			Expect(os.Symlink("/etc/hostname", filepath.Join(repo, coachConfigPath))).To(Succeed())
		}},
		{"a file where its directory should be", "a file stands where its directory should be", "cause=parent_not_directory", func(repo string) {
			Expect(os.WriteFile(filepath.Join(repo, filepath.Dir(coachConfigPath)), []byte("a file in the way"), 0o644)).To(Succeed())
		}},
	}

	for _, mode := range scopePolicyModes {
		for _, entry := range entries {
			It("#168 A2b: "+entry.description+" at the canonical path, with nothing committed there, exits 2 for what the disk holds and does not blame a submodule, under "+mode.name, func() {
				repo, base := diskPolicyRepo()
				entry.make(repo)

				stdout, stderr, exitCode := mode.run(repo, base, "--format=json")

				expectDiskJudgedRefusal(stdout, stderr, exitCode, coachConfigPath, "cannot be used on disk", entry.reason, entry.cause)
				Expect(string(stderr)).NotTo(ContainSubstring("submodule"))
			})
		}
	}
})
