//go:build unix

package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

var _ = Describe("coach codesignal reads the project policy from disk: a discovered policy on disk that cannot be used (#168 A2a, interim until A3)", func() {
	oversized := strings.Repeat(" ", int(projectconfig.MaxBytes)+1)

	type unusableOnDisk struct {
		description string
		reason      string
		make        func(repo string)
	}
	replaceWith := func(body string) func(repo string) {
		return func(repo string) { overwriteOnDisk(repo, coachConfigPath, body) }
	}
	inPlaceOfCommitted := func(create func(path string)) func(repo string) {
		return func(repo string) {
			path := filepath.Join(repo, coachConfigPath)
			Expect(os.Remove(path)).To(Succeed())
			create(path)
		}
	}
	committedCopyScenarios := []unusableOnDisk{
		{"invalid content", "not a valid project policy", replaceWith("{bad")},
		{"content over the size limit", "larger than the policy size limit", replaceWith(oversized)},
		{"a directory", "not a regular file", inPlaceOfCommitted(func(path string) { Expect(os.Mkdir(path, 0o755)).To(Succeed()) })},
		{"a symbolic link", "a symbolic link", inPlaceOfCommitted(func(path string) { Expect(os.Symlink("/etc/hostname", path)).To(Succeed()) })},
		{"a named pipe", "not a regular file", inPlaceOfCommitted(func(path string) { Expect(syscall.Mkfifo(path, 0o644)).To(Succeed()) })},
	}

	for _, mode := range scopePolicyModes {
		for _, scenario := range committedCopyScenarios {
			It("#168 item 11 (interim until A3): a committed canonical policy replaced on disk by "+scenario.description+" is replaced by the committed copy under "+mode.name+", disclosed with the reason", func() {
				repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, goLayerPolicyConfigJSON})
				scenario.make(repo)

				report, jsonOut := diskPolicyJSONScan(mode, repo, base)

				Expect(diskPolicyLayerViolations(report)).To(HaveLen(1), "the committed copy must be used")
				Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(coachConfigPath, goLayerPolicyConfigJSON, false))))
				text := diskPolicyTextScan(mode, repo, base)
				expectDiskPolicyDisclosure(mode, text,
					diskPolicyLine(coachConfigPath, goLayerPolicyConfigJSON, "differs from HEAD."),
					"WARNING: the project policy on disk cannot be used ("+scenario.reason+"); this scan used the committed copy.")
				Expect(diagnosticsOfKind(report.Diagnostics, "project_policy_unusable_on_disk")).To(BeEmpty())
			})
		}

		It("#168 item 11: a canonical policy that is not committed and is too large on disk is not used, beside a valid project.json that is not committed either, and the scan reports it by the canonical path, under "+mode.name, func() {
			repo, base := diskPolicyRepo()
			overwriteOnDiskCreatingParents(repo, coachConfigPath, oversized)
			overwriteOnDisk(repo, legacyConfigPath, diskPolicyWithoutForbiddenImports)

			report, jsonOut := diskPolicyJSONScan(mode, repo, base)

			Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", BeNil()))
			unusable := diagnosticsOfKind(report.Diagnostics, "project_policy_unusable_on_disk")
			Expect(unusable).To(HaveLen(1))
			Expect(unusable[0].Path).To(Equal(coachConfigPath))
		})

		It("#168 item 11: a root project.json that is not committed and is too large, with no canonical policy, is reported by its own path under "+mode.name, func() {
			repo, base := diskPolicyRepo()
			overwriteOnDisk(repo, legacyConfigPath, oversized)

			report, _ := diskPolicyJSONScan(mode, repo, base)

			unusable := diagnosticsOfKind(report.Diagnostics, "project_policy_unusable_on_disk")
			Expect(unusable).To(HaveLen(1))
			Expect(unusable[0].Path).To(Equal(legacyConfigPath))
		})

		It("#168 item 11: a committed canonical policy replaced on disk by a directory is not served by the root project.json that is there, under "+mode.name, func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, goLayerPolicyConfigJSON})
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())
			Expect(os.Mkdir(filepath.Join(repo, coachConfigPath), 0o755)).To(Succeed())
			overwriteOnDisk(repo, legacyConfigPath, diskPolicyWithoutForbiddenImports)

			report, jsonOut := diskPolicyJSONScan(mode, repo, base)

			Expect(diskPolicyLayerViolations(report)).To(HaveLen(1), "the committed canonical copy must be used")
			Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(coachConfigPath, goLayerPolicyConfigJSON, false))))
		})
	}
})
