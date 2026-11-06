package main

import (
	"fmt"

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
	policyReadFromCommit := func(head string) string {
		return fmt.Sprintf("the project policy is read from the commit at %s; --worktree does not read an uncommitted policy yet", head)
	}

	DescribeTable("qualifies the verdict and exits 3, because the edited policy was not read",
		func(policyPath string, policyArgs ...string) {
			repo, head := policyRepo(policyPath)
			overwriteOnDisk(repo, policyPath, goLayerPolicyConfigJSON)
			args := append(policyArgs, "--worktree", "--fail-on-incomplete-coverage")

			text, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, args...)
			jsonOut, _, _ := runCoachCodesignalBaselineRaw(repo, append(args, "--format=json")...)

			Expect(exitCode).To(Equal(3), "stderr: %s stdout: %s", stderr, text)
			Expect(string(text)).NotTo(MatchRegexp(`(?m)^No active CodeSignal findings\.$`),
				"an unqualified all-clear would tell the reader the edited policy was applied")
			Expect(string(text)).To(ContainSubstring("the analysis is incomplete"))
			notAnalyzed := diagnosticsOfKind(decodeCoachReport(jsonOut).Diagnostics, "worktree_changes_not_analyzed")
			Expect(notAnalyzed).To(ConsistOf(And(
				HaveField("Path", policyPath),
				HaveField("Message", policyReadFromCommit(head)),
			)))
		},
		Entry("the canonical policy file, discovered", ".lousy-agents/coach.json"),
		Entry("the deprecated root policy file, discovered", "project.json"),
		Entry("a policy file named with --project-config", "project.json", "--project-config", "project.json"),
	)

	It("discloses the edit in a diff scan too", func() {
		repo, head := policyRepo(".lousy-agents/coach.json")
		overwriteOnDisk(repo, ".lousy-agents/coach.json", goLayerPolicyConfigJSON)

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, head, "--worktree", "--fail-on-incomplete-coverage", "--format=json")

		Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
		Expect(diagnosticsOfKind(decodeCoachReport(stdout).Diagnostics, "worktree_changes_not_analyzed")).To(ContainElement(And(
			HaveField("Path", ".lousy-agents/coach.json"),
			HaveField("Message", policyReadFromCommit(head)),
		)))
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
