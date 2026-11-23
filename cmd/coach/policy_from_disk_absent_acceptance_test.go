package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal reads the project policy from disk: a policy only on disk or missing from disk (#168 A2a)", func() {
	const customPolicyPath = "policies/custom.json"

	for _, mode := range scopePolicyModes {
		It("#168 item 4 and item 11: a discovered canonical policy that is only on disk is used under "+mode.name+", disclosed as not committed, with no project_config_uncommitted", func() {
			repo, base := diskPolicyRepo()
			overwriteOnDiskCreatingParents(repo, coachConfigPath, goLayerPolicyConfigJSON)
			expectGitStatusContains(repo, "?? "+coachConfigPath)

			report, jsonOut := diskPolicyJSONScan(mode, repo, base)

			Expect(diskPolicyLayerViolations(report)).To(HaveLen(1), "the untracked policy must be used")
			Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(coachConfigPath, goLayerPolicyConfigJSON, diskCauseNotCommitted))))
			Expect(diagnosticsOfKind(report.Diagnostics, "project_config_uncommitted")).To(BeEmpty())
			text := diskPolicyTextScan(mode, repo, base)
			expectDiskPolicyDisclosure(mode, text, diskPolicyLine(coachConfigPath, goLayerPolicyConfigJSON, "not committed at HEAD."), diskPolicyNotCommittedWarning(coachConfigPath))
		})

		It("#168 item 11: a policy named with --project-config that is an untracked regular file is read under "+mode.name+", not refused with exit 2", func() {
			repo, base := diskPolicyRepo()
			overwriteOnDiskCreatingParents(repo, customPolicyPath, goLayerPolicyConfigJSON)
			expectGitStatusContains(repo, "?? "+customPolicyPath)

			report, jsonOut := diskPolicyJSONScan(mode, repo, base, "--project-config", customPolicyPath)

			Expect(diskPolicyLayerViolations(report)).To(HaveLen(1), "the untracked policy must be used")
			Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(customPolicyPath, goLayerPolicyConfigJSON, diskCauseNotCommitted))))
			Expect(diagnosticsOfKind(report.Diagnostics, "project_config_uncommitted")).To(BeEmpty())
			text := diskPolicyTextScan(mode, repo, base, "--project-config", customPolicyPath)
			expectDiskPolicyDisclosure(mode, text, diskPolicyLine(customPolicyPath, goLayerPolicyConfigJSON, "not committed at HEAD."), diskPolicyNotCommittedWarning(customPolicyPath))
		})

		It("#168 item 4: an untracked canonical policy beside a committed root project.json is the policy used under "+mode.name+", and is not matches_head", func() {
			repo, base := diskPolicyRepo(scopePolicyFile{legacyConfigPath, diskPolicyWithoutForbiddenImports})
			overwriteOnDiskCreatingParents(repo, coachConfigPath, goLayerPolicyConfigJSON)
			expectGitStatusContains(repo, "?? "+coachConfigPath)

			report, jsonOut := diskPolicyJSONScan(mode, repo, base)

			Expect(diskPolicyLayerViolations(report)).To(HaveLen(1), "the canonical file on disk wins over the committed project.json")
			Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(coachConfigPath, goLayerPolicyConfigJSON, diskCauseNotCommitted))))
			text := diskPolicyTextScan(mode, repo, base)
			expectDiskPolicyDisclosure(mode, text, diskPolicyLine(coachConfigPath, goLayerPolicyConfigJSON, "not committed at HEAD."), diskPolicyNotCommittedWarning(coachConfigPath))
		})

		It("#168 item 4 and item 11 (interim until A3): a discovered policy committed at HEAD and deleted on disk uses the committed copy under "+mode.name+", disclosed as not matches_head with the missing-on-disk WARNING", func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, goLayerPolicyConfigJSON})
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())
			expectGitStatusContains(repo, " D "+coachConfigPath)

			report, jsonOut := diskPolicyJSONScan(mode, repo, base)

			Expect(diskPolicyLayerViolations(report)).To(HaveLen(1), "the committed copy must be used")
			Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(coachConfigPath, goLayerPolicyConfigJSON, diskCauseMissingOnDisk))))
			text := diskPolicyTextScan(mode, repo, base)
			expectDiskPolicyDisclosure(mode, text, diskPolicyLine(coachConfigPath, goLayerPolicyConfigJSON, "differs from HEAD."), diskPolicyMissingOnDiskWarning(coachConfigPath))
			Expect(string(text)).NotTo(ContainSubstring(diskPolicyDiffersWarning))
		})

		It("#168 item 11: a policy named with --project-config that is committed and deleted on disk still exits 2 under "+mode.name, func() {
			repo, base := diskPolicyRepo(scopePolicyFile{customPolicyPath, goLayerPolicyConfigJSON})
			Expect(os.Remove(filepath.Join(repo, customPolicyPath))).To(Succeed())
			expectGitStatusContains(repo, " D "+customPolicyPath)

			_, _, exitCode := mode.run(repo, base, "--project-config", customPolicyPath, "--format=json")

			Expect(exitCode).To(Equal(2))
		})
	}
})
