package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

const permissivePolicyJSON = `{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]}]}`

var _ = Describe("coach codesignal --worktree for a policy-named file beside the selected policy", func() {
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
	scan := func(repo string) (report *codesignal.Report, stderr string, exitCode int) {
		stdout, stderrBytes, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--fail-on-incomplete-coverage", "--format=json")
		return decodeCoachReport(stdout), string(stderrBytes), exitCode
	}

	DescribeTable("names it once, as a change the scan did not read, without a claim that the policy comes from the commit",
		func(change func(repo string), wantNamed string) {
			repo, _ := repoCommitting(".lousy-agents/coach.json", "project.json")
			change(repo)

			report, stderr, exitCode := scan(repo)

			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
			Expect(diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).To(BeEmpty())
			Expect(diagnosticsOfKind(report.Diagnostics, "worktree_not_clean")).To(ConsistOf(SatisfyAll(
				HaveField("Message", HavePrefix("working-tree changes this scan did not read: ")),
				HaveField("Message", ContainSubstring(wantNamed)),
				HaveField("Message", Not(ContainSubstring("from the commit"))),
				HaveField("Message", WithTransform(func(message string) int { return strings.Count(message, "project.json") }, BeNumerically("<=", 1))),
			)))
			for _, diagnostic := range report.Diagnostics {
				Expect(diagnostic.Message).NotTo(ContainSubstring("from the commit"))
			}
		},
		Entry("a committed deprecated policy beside the selected canonical one, edited", func(repo string) {
			overwriteOnDisk(repo, "project.json", goLayerPolicyConfigJSON)
		}, "project.json"),
		Entry("a committed deprecated policy beside the selected canonical one, renamed away", func(repo string) {
			runGitInRepo(repo, "mv", "project.json", "moved.json")
		}, "moved.json, project.json"),
	)

	DescribeTable("reads a canonical policy that is not committed from disk in preference to the committed deprecated one, and discloses it as not committed",
		func(change func(repo string)) {
			repo, _ := repoCommitting("project.json")
			change(repo)

			stdout, stderrBytes, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--fail-on-incomplete-coverage", "--format=json")
			text, _, _ := runCoachCodesignalBaselineRaw(repo, "--worktree", "--fail-on-incomplete-coverage")

			Expect(exitCode).To(Equal(0), "stderr: %s", stderrBytes)
			report := decodeCoachReport(stdout)
			Expect(diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).To(BeEmpty())
			Expect(diagnosticsOfKind(report.Diagnostics, "worktree_not_clean")).To(BeEmpty())
			Expect(scopePolicyScopeOf(stdout)).To(HaveKeyWithValue("policy", Equal(diskPolicyScope(".lousy-agents/coach.json", permissivePolicyJSON, diskCauseNotCommitted))))
			Expect(string(text)).To(ContainSubstring(diskPolicyLine(".lousy-agents/coach.json", permissivePolicyJSON, "not committed at HEAD.")))
			Expect(string(text)).To(ContainSubstring(diskPolicyNotCommittedWarning(".lousy-agents/coach.json")))
		},
		Entry("staged", func(repo string) {
			overwriteOnDiskCreatingParents(repo, ".lousy-agents/coach.json", permissivePolicyJSON)
			runGitInRepo(repo, "add", ".lousy-agents/coach.json")
		}),
		Entry("untracked", func(repo string) {
			overwriteOnDiskCreatingParents(repo, ".lousy-agents/coach.json", permissivePolicyJSON)
		}),
	)
})
