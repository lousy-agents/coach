package main

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A policy renamed out of its path is still the policy the scan reads from the
// commit, so a scan that said nothing about the rename would end in an all-clear
// for a policy the user has already moved. The deprecated file's rename-out is
// pinned in worktree_policy_wording_acceptance_test.go; this pins the canonical
// file's.
var _ = Describe("coach codesignal --worktree when the canonical project policy is renamed out of its path", func() {
	const (
		canonicalPolicyPath  = ".lousy-agents/coach.json"
		deprecatedPolicyPath = "project.json"
		renamedPolicyPath    = "moved.json"
	)

	readFromCommit := func(head string) string {
		return fmt.Sprintf("the project policy is read from the commit at %s; --worktree does not read an uncommitted policy yet", head)
	}
	hasChanges := func(head string) string {
		return fmt.Sprintf("the project policy file at %s has uncommitted changes; --worktree reads the policy from the commit at %s and does not read an uncommitted policy yet", canonicalPolicyPath, head)
	}
	repoCommitting := func(policies ...string) (repo, head string) {
		repo = newTempGitRepo()
		commitFile(repo, "go.mod", goModuleFile)
		commitFile(repo, "pkg/db/db.go", dbPackageFile)
		head = commitFile(repo, "pkg/handlers/handlers.go", handlersImportingDB)
		for _, policy := range policies {
			head = commitFile(repo, policy, permissivePolicyJSON)
		}
		return repo, head
	}

	DescribeTable("names the old canonical path, exits 3, and gives no all-clear",
		func(committed, scanArgs []string, wording func(head string) string) {
			repo, head := repoCommitting(committed...)
			runGitInRepo(repo, "mv", canonicalPolicyPath, renamedPolicyPath)
			args := append(scanArgs, "--worktree", "--fail-on-incomplete-coverage")

			text, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, args...)
			jsonOut, _, _ := runCoachCodesignalBaselineRaw(repo, append(args, "--format=json")...)

			Expect(exitCode).To(Equal(3), "stderr: %s stdout: %s", stderr, text)
			Expect(string(text)).NotTo(MatchRegexp(`(?m)^No active CodeSignal findings\.$`),
				"an unqualified all-clear would tell the reader the policy they renamed was applied")
			Expect(string(text)).To(ContainSubstring("the analysis is incomplete"))
			report := decodeCoachReport(jsonOut)
			Expect(diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).To(ConsistOf(And(
				HaveField("Path", canonicalPolicyPath),
				HaveField("Message", wording(head)),
			)))
			Expect(diagnosticsOfKind(report.Diagnostics, "worktree_not_clean")).NotTo(ContainElement(HaveField("Message", ContainSubstring(canonicalPolicyPath))),
				"the policy already has its own diagnostic, so listing it again would count it twice")
		},
		Entry("the selected canonical policy, renamed with git mv",
			[]string{canonicalPolicyPath}, []string(nil), readFromCommit),
		Entry("a canonical policy that --project-config did not select, renamed with git mv",
			[]string{canonicalPolicyPath, deprecatedPolicyPath}, []string{"--project-config", deprecatedPolicyPath}, hasChanges),
	)

	It("qualifies a diff scan the same way", func() {
		repo, head := repoCommitting(canonicalPolicyPath)
		runGitInRepo(repo, "mv", canonicalPolicyPath, renamedPolicyPath)

		stdout, stderr, exitCode := runCoachCodesignalRaw(repo, head, "--worktree", "--fail-on-incomplete-coverage", "--format=json")

		Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
		Expect(diagnosticsOfKind(decodeCoachReport(stdout).Diagnostics, "worktree_changes_not_analyzed")).To(ContainElement(And(
			HaveField("Path", canonicalPolicyPath),
			HaveField("Message", readFromCommit(head)),
		)))
	})

	It("keeps the all-clear and exits 0 when the committed canonical policy stays where it is (control)", func() {
		repo, _ := repoCommitting(canonicalPolicyPath)

		text, stderr, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--fail-on-incomplete-coverage")

		Expect(exitCode).To(Equal(0), "stderr: %s stdout: %s", stderr, text)
		Expect(noFindingsHeadline(string(text))).To(Equal("No active CodeSignal findings."))
	})
})
