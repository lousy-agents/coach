package main

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --worktree for a project policy it reads from the commit", func() {
	notCommitted := func(path, head string) string {
		return fmt.Sprintf("the project policy at %s is not committed; --worktree reads the policy from the commit at %s and does not read an uncommitted policy yet", path, head)
	}
	repoWithoutPolicy := func() (repo, head string) {
		repo = newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/db/db.go", dbPackageFile)
		head = commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
		return repo, head
	}

	DescribeTable("names an untracked policy file and exits 3, instead of applying no policy and exiting 0",
		func(untrackedPolicy string) {
			repo, head := repoWithoutPolicy()
			overwriteOnDiskCreatingParents(repo, untrackedPolicy, goLayerPolicyConfigJSON)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--fail-on-incomplete-coverage", "--format=json")

			Expect(exitCode).To(Equal(3), "stderr: %s stdout: %s", stderr, stdout)
			named := diagnosticsOfKind(decodeCoachReport(stdout).Diagnostics, "worktree_changes_not_analyzed")
			Expect(named).To(ConsistOf(And(
				HaveField("Path", untrackedPolicy),
				HaveField("Message", notCommitted(untrackedPolicy, head)),
			)))
		},
		Entry("the canonical policy file", ".lousy-agents/coach.json"),
		Entry("the deprecated root policy file", "project.json"),
	)

	DescribeTable("names a policy file at every status, not only untracked, and exits 3",
		func(stage func(repo string), policyPath string) {
			repo, _ := repoWithoutPolicy()
			commitFile(repo, "draft.json", goLayerPolicyConfigJSON)
			stage(repo)

			stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--fail-on-incomplete-coverage", "--format=json")

			Expect(exitCode).To(Equal(3), "stderr: %s stdout: %s", stderr, stdout)
			Expect(diagnosticsOfKind(decodeCoachReport(stdout).Diagnostics, "worktree_changes_not_analyzed")).To(ContainElement(HaveField("Path", policyPath)),
				"a policy the scan did not read must qualify the verdict whatever its status code")
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
		}, "project.json"),
		Entry("the committed policy renamed to another name", func(repo string) {
			commitFile(repo, "project.json", goLayerPolicyConfigJSON)
			runGitInRepo(repo, "mv", "project.json", "moved.json")
		}, "project.json"),
	)

	It("names the untracked canonical policy beside a committed deprecated one that is selected", func() {
		repo, head := repoWithoutPolicy()
		commitFile(repo, "project.json", goLayerPolicyConfigJSON)
		head = commitFile(repo, "pkg/db/extra.go", "package db\n")
		overwriteOnDiskCreatingParents(repo, ".lousy-agents/coach.json", goLayerPolicyConfigJSON)

		stdout, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--fail-on-incomplete-coverage", "--format=json")

		Expect(exitCode).To(Equal(3), "stderr: %s stdout: %s", stderr, stdout)
		Expect(diagnosticsOfKind(decodeCoachReport(stdout).Diagnostics, "worktree_changes_not_analyzed")).To(ConsistOf(And(
			HaveField("Path", ".lousy-agents/coach.json"),
			HaveField("Message", notCommitted(".lousy-agents/coach.json", head)),
		)))
	})

	It("keeps the note that the canonical policy file is not committed", func() {
		repo, _ := repoWithoutPolicy()
		overwriteOnDiskCreatingParents(repo, ".lousy-agents/coach.json", goLayerPolicyConfigJSON)

		stdout, _, _ := runCoachCodesignalBaselineRaw(repo, "--worktree", "--format=json")

		Expect(diagnosticsOfKind(decodeCoachReport(stdout).Diagnostics, "project_config_uncommitted")).To(HaveLen(1))
	})

	It("names an edited policy once, and does not also call the working tree not clean", func() {
		repo := newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/db/db.go", dbPackageFile)
		commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
		commitFile(repo, ".lousy-agents/coach.json", `{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]}]}`)
		overwriteOnDisk(repo, ".lousy-agents/coach.json", goLayerPolicyConfigJSON)

		text, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--fail-on-incomplete-coverage")
		jsonOut, _, _ := runCoachCodesignalBaselineRaw(repo, "--worktree", "--fail-on-incomplete-coverage", "--format=json")

		Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
		Expect(string(text)).NotTo(ContainSubstring("working tree is not clean"),
			"--worktree exists to scan a dirty tree; the committed-mode wording reads as 'your changes were ignored'")
		Expect(diagnosticsOfKind(decodeCoachReport(jsonOut).Diagnostics, "worktree_not_clean")).To(BeEmpty(),
			"the policy already has its own worktree_changes_not_analyzed diagnostic")
		Expect(diagnosticsOfKind(decodeCoachReport(jsonOut).Diagnostics, "worktree_changes_not_analyzed")).To(HaveLen(1))
	})
})
