package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree for a project policy that is not committed", func() {
	worktreeBaseline := scopePolicyModes[1]
	repoWithoutPolicy := func() (repo, head string) {
		repo = newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/db/db.go", dbPackageFile)
		head = commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
		return repo, head
	}

	DescribeTable("reads an untracked policy file from disk, discloses it as not committed, and exits 0 although coverage is required to be complete",
		func(untrackedPolicy string) {
			repo, _ := repoWithoutPolicy()
			overwriteOnDiskCreatingParents(repo, untrackedPolicy, goLayerPolicyConfigJSON)

			text, stderr, exitCode := worktreeBaseline.run(repo, "", "--fail-on-incomplete-coverage")
			jsonOut, _, _ := worktreeBaseline.run(repo, "", "--fail-on-incomplete-coverage", "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, text)
			report := decodeCoachReport(jsonOut)
			Expect(diskPolicyLayerViolations(report)).To(HaveLen(1), "the untracked policy is applied")
			Expect(diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).To(BeEmpty())
			Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(untrackedPolicy, goLayerPolicyConfigJSON, diskCauseNotCommitted))))
			expectDiskPolicyDisclosure(worktreeBaseline, text, diskPolicyLine(untrackedPolicy, goLayerPolicyConfigJSON, "not committed at HEAD."), diskPolicyNotCommittedWarning(untrackedPolicy))
		},
		Entry("the canonical policy file", ".lousy-agents/coach.json"),
		Entry("the deprecated root policy file", "project.json"),
	)

	DescribeTable("reads a policy file staged in any way from disk, discloses it as not committed, and exits 0",
		func(stage func(repo string), policyPath string) {
			repo, _ := repoWithoutPolicy()
			commitFile(repo, "draft.json", goLayerPolicyConfigJSON)
			stage(repo)

			text, stderr, exitCode := worktreeBaseline.run(repo, "", "--fail-on-incomplete-coverage")
			jsonOut, _, _ := worktreeBaseline.run(repo, "", "--fail-on-incomplete-coverage", "--format=json")

			Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, text)
			report := decodeCoachReport(jsonOut)
			Expect(diskPolicyLayerViolations(report)).To(HaveLen(1), "the staged policy is applied")
			Expect(diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).To(BeEmpty(),
				"a policy the scan read from disk went unread under no status code")
			Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(policyPath, goLayerPolicyConfigJSON, diskCauseNotCommitted))))
			expectDiskPolicyDisclosure(worktreeBaseline, text, diskPolicyLine(policyPath, goLayerPolicyConfigJSON, "not committed at HEAD."), diskPolicyNotCommittedWarning(policyPath))
		},
		Entry("a staged new policy file", func(repo string) {
			overwriteOnDisk(repo, "project.json", goLayerPolicyConfigJSON)
			runGitInRepo(repo, "add", "project.json")
		}, "project.json"),
		Entry("an intent-to-add policy file", func(repo string) {
			overwriteOnDisk(repo, "project.json", goLayerPolicyConfigJSON)
			runGitInRepo(repo, "add", "--intent-to-add", "project.json")
		}, "project.json"),
		Entry("a file renamed into the policy path", func(repo string) {
			runGitInRepo(repo, "mv", "draft.json", "project.json")
		}, "project.json"),
		Entry("a staged canonical policy beside a committed deprecated one", func(repo string) {
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			overwriteOnDiskCreatingParents(repo, ".lousy-agents/coach.json", goLayerPolicyConfigJSON)
			runGitInRepo(repo, "add", ".lousy-agents/coach.json")
		}, ".lousy-agents/coach.json"),
		Entry("the committed policy renamed to the canonical path", func(repo string) {
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			overwriteOnDiskCreatingParents(repo, ".lousy-agents/.keep", "")
			runGitInRepo(repo, "mv", "project.json", ".lousy-agents/coach.json")
		}, ".lousy-agents/coach.json"),
	)

	It("#168 item 3: is file-local when the committed policy is renamed to another name, because no policy file is on disk, and says so", func() {
		repo, _ := repoWithoutPolicy()
		commitFile(repo, "project.json", goLayerPolicyConfigJSON)
		runGitInRepo(repo, "mv", "project.json", "moved.json")

		text, stderr, exitCode := worktreeBaseline.run(repo, "", "--fail-on-incomplete-coverage")
		jsonOut, _, _ := worktreeBaseline.run(repo, "", "--fail-on-incomplete-coverage", "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, text)
		Expect(diagnosticsOfKind(decodeCoachReport(jsonOut).Diagnostics, "worktree_changes_not_analyzed")).To(BeEmpty())
		Expect(scopePolicyScopeOf(jsonOut)).To(HaveKeyWithValue("policy", BeNil()))
		Expect(warningLinesNaming(text, "project.json")).NotTo(BeEmpty(), "a WARNING must name the missing policy; text:\n%s", text)
		Expect(string(text)).NotTo(ContainSubstring(diskPolicyMissingOnDiskWarning("project.json")))
	})

	It("reads the untracked canonical policy beside a committed deprecated one, and does not report it as unread", func() {
		repo, _ := repoWithoutPolicy()
		commitFile(repo, "project.json", permissivePolicyJSON)
		commitFile(repo, "pkg/db/extra.go", "package db\n")
		overwriteOnDiskCreatingParents(repo, ".lousy-agents/coach.json", goLayerPolicyConfigJSON)

		stdout, stderr, exitCode := worktreeBaseline.run(repo, "", "--fail-on-incomplete-coverage", "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, stdout)
		report := decodeCoachReport(stdout)
		Expect(diskPolicyLayerViolations(report)).To(HaveLen(1), "the canonical file on disk wins over the committed project.json")
		Expect(diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).To(BeEmpty())
		Expect(scopePolicyScopeOf(stdout)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(".lousy-agents/coach.json", goLayerPolicyConfigJSON, diskCauseNotCommitted))))
	})

	It("does not emit project_config_uncommitted for the untracked canonical policy, which scope.policy discloses instead", func() {
		repo, _ := repoWithoutPolicy()
		overwriteOnDiskCreatingParents(repo, ".lousy-agents/coach.json", goLayerPolicyConfigJSON)

		stdout, _, _ := worktreeBaseline.run(repo, "", "--format=json")

		Expect(diagnosticsOfKind(decodeCoachReport(stdout).Diagnostics, "project_config_uncommitted")).To(BeEmpty())
		Expect(scopePolicyScopeOf(stdout)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(".lousy-agents/coach.json", goLayerPolicyConfigJSON, diskCauseNotCommitted))))
	})

	It("reads an edited policy, and neither names it as unread nor calls the working tree not clean", func() {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/db/db.go", dbPackageFile)
		commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
		commitFile(repo, ".lousy-agents/coach.json", `{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]}]}`)
		overwriteOnDisk(repo, ".lousy-agents/coach.json", goLayerPolicyConfigJSON)

		text, stderr, exitCode := worktreeBaseline.run(repo, "", "--fail-on-incomplete-coverage")
		jsonOut, _, _ := worktreeBaseline.run(repo, "", "--fail-on-incomplete-coverage", "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(string(text)).NotTo(ContainSubstring("working tree is not clean"),
			"--worktree exists to scan a dirty tree; the committed-mode wording reads as 'your changes were ignored'")
		Expect(diagnosticsOfKind(decodeCoachReport(jsonOut).Diagnostics, "worktree_not_clean")).To(BeEmpty())
		Expect(diagnosticsOfKind(decodeCoachReport(jsonOut).Diagnostics, "worktree_changes_not_analyzed")).To(BeEmpty(),
			"the policy was read from disk, so no edit to it went unread")
	})
})
