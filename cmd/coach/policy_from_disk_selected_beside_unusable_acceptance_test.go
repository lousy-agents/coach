//go:build unix

package main

import (
	"os"
	"path/filepath"
	"syscall"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal reads the project policy from disk: a canonical policy on disk beside an unusable committed root project.json", func() {
	type entryInPlace struct {
		description string
		make        func(path string)
	}
	entries := []entryInPlace{
		{"a directory", func(path string) { Expect(os.Mkdir(path, 0o755)).To(Succeed()) }},
		{"a named pipe", func(path string) { Expect(syscall.Mkfifo(path, 0o644)).To(Succeed()) }},
	}

	for _, mode := range scopePolicyModes {
		for _, entry := range entries {
			It("uses the canonical policy on disk when the committed root project.json is replaced by "+entry.description+" under "+mode.name, func() {
				repo, base := diskPolicyRepo(scopePolicyFile{legacyConfigPath, diskPolicyWithoutForbiddenImports})
				path := filepath.Join(repo, legacyConfigPath)
				Expect(os.Remove(path)).To(Succeed())
				entry.make(path)
				overwriteOnDiskCreatingParents(repo, coachConfigPath, goLayerPolicyConfigJSON)

				report, jsonOut := diskPolicyJSONScan(mode, repo, base)

				Expect(diskPolicyLayerViolations(report)).To(HaveLen(1), "the canonical policy on disk must be applied")
				Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(coachConfigPath, goLayerPolicyConfigJSON, diskCauseNotCommitted))))
				text := diskPolicyTextScan(mode, repo, base)
				expectDiskPolicyDisclosure(mode, text,
					diskPolicyLine(coachConfigPath, goLayerPolicyConfigJSON, "not committed at HEAD."),
					diskPolicyNotCommittedWarning(coachConfigPath))
				Expect(diagnosticsOfKind(report.Diagnostics, "project_policy_unusable_on_disk")).To(BeEmpty())
			})
		}
	}
})
