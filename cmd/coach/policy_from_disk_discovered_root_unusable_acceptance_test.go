//go:build unix

package main

import (
	"os"
	"path/filepath"
	"syscall"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal reads the project policy from disk: a committed root project.json whose path holds an entry that cannot be used (#168 A2b)", func() {
	type entryInPlace struct {
		description string
		cause       string
		make        func(path string)
	}
	entries := []entryInPlace{
		{"a directory", "not_a_regular_file", func(path string) { Expect(os.Mkdir(path, 0o755)).To(Succeed()) }},
		{"a symbolic link", "symbolic_link", func(path string) { Expect(os.Symlink("/etc/hostname", path)).To(Succeed()) }},
		{"a named pipe", "not_a_regular_file", func(path string) { Expect(syscall.Mkfifo(path, 0o644)).To(Succeed()) }},
	}

	for _, mode := range scopePolicyModes {
		for _, entry := range entries {
			It("#168 A2b: a committed root project.json replaced on disk by "+entry.description+" exits 2 as project_config_invalid under "+mode.name+", and the committed copy does not stand in", func() {
				repo, base := diskPolicyRepo(scopePolicyFile{legacyConfigPath, goLayerPolicyConfigJSON})
				path := filepath.Join(repo, legacyConfigPath)
				Expect(os.Remove(path)).To(Succeed())
				entry.make(path)

				stdout, stderr, exitCode := mode.run(repo, base, "--format=json")

				expectDiskJudgedRefusal(stdout, stderr, exitCode, legacyConfigPath, entry.cause)
				Expect(string(stderr)).NotTo(ContainSubstring("unusable_on_disk"))
			})
		}

		It("#168 A2b: a root project.json that is not committed and is a named pipe on disk keeps its behavior under "+mode.name+": no policy is used and none is reported", func() {
			repo, base := diskPolicyRepo()
			Expect(syscall.Mkfifo(filepath.Join(repo, legacyConfigPath), 0o644)).To(Succeed())

			report, jsonOut := diskPolicyJSONScan(mode, repo, base)

			Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", BeNil()))
			Expect(diagnosticsOfKind(report.Diagnostics, "project_policy_unusable_on_disk")).To(BeEmpty())
		})
	}
})
