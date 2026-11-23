package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree discloses an uncommitted edit to the selected project policy", func() {
	const policyWithoutForbiddenImports = `{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]}]}`

	policyRepo := func(policyPath string) (repo, head string) {
		repo = newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/db/db.go", dbPackageFile)
		commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
		head = commitFile(repo, policyPath, policyWithoutForbiddenImports)
		return repo, head
	}
	worktreeBaseline := scopePolicyModes[1]

	DescribeTable("reads the edited policy, discloses that it differs from HEAD, and exits 0 although coverage is required to be complete",
		func(policyPath string, policyArgs ...string) {
			repo, _ := policyRepo(policyPath)
			overwriteOnDisk(repo, policyPath, goLayerPolicyConfigJSON)
			args := append(append([]string{}, policyArgs...), "--fail-on-incomplete-coverage")

			text, stderr, exitCode := worktreeBaseline.run(repo, "", args...)
			jsonOut, _, _ := worktreeBaseline.run(repo, "", append(args, "--format=json")...)

			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, text)
			report := decodeCoachReport(jsonOut)
			Expect(diskPolicyLayerViolations(report)).To(HaveLen(1), "the edited policy is applied")
			Expect(diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).To(BeEmpty(), "the policy file was read from disk")
			Expect(string(text)).NotTo(ContainSubstring("the analysis is incomplete"))
			Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(policyPath, goLayerPolicyConfigJSON, diskCauseDiffersFromHead))))
			expectDiskPolicyDisclosure(worktreeBaseline, text, diskPolicyLine(policyPath, goLayerPolicyConfigJSON, "differs from HEAD."), diskPolicyDiffersWarning)
		},
		Entry("the canonical policy file, discovered", ".lousy-agents/coach.json"),
		Entry("the deprecated root policy file, discovered", "project.json"),
		Entry("a policy file named with --project-config", "project.json", "--project-config", "project.json"),
	)

	It("reads the edited policy in a diff scan too", func() {
		repo, head := policyRepo(".lousy-agents/coach.json")
		overwriteOnDisk(repo, ".lousy-agents/coach.json", goLayerPolicyConfigJSON)

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, head, "--worktree", "--fail-on-incomplete-coverage", "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		report := decodeCoachReport(stdout)
		Expect(diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).To(BeEmpty())
		Expect(scopePolicyScopeOf(stdout)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(".lousy-agents/coach.json", goLayerPolicyConfigJSON, diskCauseDiffersFromHead))))
	})

	It("keeps the all-clear and exits 0 when the policy is unchanged (control)", func() {
		repo, _ := policyRepo(".lousy-agents/coach.json")

		text, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--fail-on-incomplete-coverage")

		Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, text)
		Expect(noFindingsHeadline(string(text))).To(Equal("No active CodeSignal findings."))
	})

	It("does not claim the policy was left unread when only another file is dirty (control)", func() {
		repo, _ := policyRepo(".lousy-agents/coach.json")
		overwriteOnDisk(repo, "pkg/handlers/handlers.go", handlersImportingDB+"\n// edited\n")

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--fail-on-incomplete-coverage", "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(diagnosticsOfKind(decodeCoachReport(stdout).Diagnostics, "worktree_changes_not_analyzed")).To(BeEmpty())
	})
})
