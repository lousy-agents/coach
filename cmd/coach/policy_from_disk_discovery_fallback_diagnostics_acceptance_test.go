package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func diagnosticsListingPath(report *codesignal.Report, kind, path string) []codesignal.Diagnostic {
	var listing []codesignal.Diagnostic
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Kind == kind && (diagnostic.Path == path || strings.Contains(diagnostic.Message, path)) {
			listing = append(listing, diagnostic)
		}
	}
	return listing
}

// expectNoUnreadListingOfDeletedPolicy covers the overlay scans only: the committed scans' own working-tree notice
// lists every dirty path.
func expectNoUnreadListingOfDeletedPolicy(mode scopePolicyMode, report *codesignal.Report) {
	GinkgoHelper()
	if len(mode.args) == 0 {
		return
	}
	Expect(diagnosticsListingPath(report, "worktree_not_clean", coachConfigPath)).To(BeEmpty(), "diagnostics: %+v", report.Diagnostics)
}

var _ = Describe("a deleted discovered policy is disclosed once, by project_policy_absent_on_disk, and is no unread change (#168 A3)", func() {
	for _, mode := range scopePolicyModes {
		It("#168 item 2: under "+mode.name+", the fallback to project.json names the absent canonical policy and the deprecation, and lists the deletion nowhere else", func() {
			repo, base := diskPolicyRepo(
				scopePolicyFile{coachConfigPath, diskPolicyWithoutForbiddenImports},
				scopePolicyFile{legacyConfigPath, goLayerPolicyConfigJSON},
			)
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())

			report, _ := diskPolicyJSONScan(mode, repo, base)

			expectDiagnosticKind(report, "project_policy_absent_on_disk", ".lousy-agents/coach.json")
			expectDiagnosticKind(report, "project_config_deprecated", "project.json")
			expectNoDiagnosticKind(report, "worktree_report_reflects_committed_head")
			expectNoUnreadListingOfDeletedPolicy(mode, report)
		})

		It("#168 item 3: under "+mode.name+", the file-local scan names the absent canonical policy, no deprecation, and lists the deletion nowhere else", func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, goLayerPolicyConfigJSON})
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())

			report, _ := diskPolicyJSONScan(mode, repo, base)

			expectDiagnosticKind(report, "project_policy_absent_on_disk", ".lousy-agents/coach.json")
			expectNoDiagnosticKind(report, "project_config_deprecated")
			expectNoDiagnosticKind(report, "worktree_report_reflects_committed_head")
			expectNoUnreadListingOfDeletedPolicy(mode, report)
		})

		It("#168 item 2: under "+mode.name+", a committed root project.json left alone on disk is reported as deprecated exactly once", func() {
			repo, base := diskPolicyRepo(scopePolicyFile{legacyConfigPath, goLayerPolicyConfigJSON})

			report, _ := diskPolicyJSONScan(mode, repo, base)
			text := diskPolicyTextScan(mode, repo, base)

			Expect(diagnosticsOfKind(report.Diagnostics, "project_config_deprecated")).To(HaveLen(1), "diagnostics: %+v", report.Diagnostics)
			Expect(strings.Count(string(text), "kind: project_config_deprecated")).To(Equal(1), "text:\n%s", text)
		})

		It("#168 item 3: under "+mode.name+", with no policy committed or on disk, no absent-policy diagnostic is reported", func() {
			repo, base := diskPolicyRepo()

			report, _ := diskPolicyJSONScan(mode, repo, base)

			expectNoDiagnosticKind(report, "project_policy_absent_on_disk")
		})

		It("#168 item 2: under "+mode.name+", a root project.json named with --project-config is no deprecated fallback", func() {
			repo, base := diskPolicyRepo(scopePolicyFile{legacyConfigPath, goLayerPolicyConfigJSON})

			report, _ := diskPolicyJSONScan(mode, repo, base, "--project-config", legacyConfigPath)

			expectNoDiagnosticKind(report, "project_config_deprecated")
			expectNoDiagnosticKind(report, "project_policy_absent_on_disk")
		})

		It("#168 item 3: under "+mode.name+", a committed root project.json deleted on disk is file-local with no deprecation and no migration offer", func() {
			repo, base := diskPolicyRepo(scopePolicyFile{legacyConfigPath, goLayerPolicyConfigJSON})
			Expect(os.Remove(filepath.Join(repo, legacyConfigPath))).To(Succeed())

			report, _ := diskPolicyJSONScan(mode, repo, base)

			expectDiagnosticKind(report, "project_policy_absent_on_disk", "project.json")
			expectNoDiagnosticKind(report, "project_config_deprecated")
			expectNoDiagnosticKind(report, "project_config_migration_skipped")
			expectNoDiagnosticKind(report, "project_config_migration_failed")
		})
	}

	It("#168 item 3: on a controlling terminal, a committed root project.json deleted on disk gets no prompt to copy it", func() {
		repo := commitDeprecatedProjectConfigRepo()
		Expect(os.Remove(filepath.Join(repo, legacyConfigPath))).To(Succeed())

		stdout, stderr, exitCode := runMigrationBaselineOnTerminal(repo, projectConfigMigrationEnv(), "y\n")

		Expect(exitCode).To(Equal(0), "stderr: %s\nstdout: %s", stderr, stdout)
		Expect(string(stderr)).NotTo(ContainSubstring("Copy project.json"))
		expectRepoFileAbsent(repo, coachConfigPath)
		expectNoDiagnosticKind(decodeCoachReport(stdout), "project_config_deprecated")
	})

	It("#168 item 3: a deleted committed canonical policy leaves exactly the working-tree notice and the absent-policy diagnostic", func() {
		repo := commitScannableGoRepo()
		commitFile(repo, coachConfigPath, singleRootPolicyJSON)
		Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--format=json")

		Expect(exitCode).To(Equal(0), "stdout=%s stderr=%s", stdout, stderr)
		var kinds []string
		for _, diagnostic := range decodeCoachReport(stdout).Diagnostics {
			kinds = append(kinds, diagnostic.Kind)
		}
		Expect(kinds).To(ConsistOf("worktree_not_clean", "project_policy_absent_on_disk"))
	})
})
