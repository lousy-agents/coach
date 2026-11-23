package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

var _ = Describe("coach codesignal reads the project policy from disk: --worktree diagnostics (#168 A2a)", func() {
	for _, mode := range scopePolicyModes {
		if len(mode.args) == 0 {
			continue
		}

		It("#168 item 1: a policy file the scan read from disk is not reported as unread under --worktree, under "+mode.name, func() {
			repo, base := diskPolicyRepo(scopePolicyFile{coachConfigPath, diskPolicyWithoutForbiddenImports})
			overwriteOnDisk(repo, coachConfigPath, goLayerPolicyConfigJSON)
			expectGitStatusContains(repo, " M "+coachConfigPath)

			report, _ := diskPolicyJSONScan(mode, repo, base)

			Expect(diskPolicyLayerViolations(report)).To(HaveLen(1), "the edited policy is applied")
			Expect(diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).NotTo(ContainElement(HaveField("Path", coachConfigPath)))
			Expect(diagnosticsOfKind(report.Diagnostics, codesignal.DiagKindWorktreeReportReflectsCommittedHEAD)).To(BeEmpty(),
				"the policy edit is the only dirty path, so it is the only possible cause")
		})
	}
})
