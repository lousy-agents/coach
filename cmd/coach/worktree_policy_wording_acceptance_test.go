package main

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

const permissivePolicyJSON = `{"schema_version":"1","roots":["."],"layers":[{"name":"handlers","prefixes":["pkg/handlers"]},{"name":"db","prefixes":["pkg/db"]}]}`

var _ = Describe("coach codesignal --worktree for a policy file the scan did not select", func() {
	notCommitted := func(path, head string) string {
		return fmt.Sprintf("the project policy at %s is not committed; --worktree reads the policy from the commit at %s and does not read an uncommitted policy yet", path, head)
	}
	hasChanges := func(path, head string) string {
		return fmt.Sprintf("the project policy file at %s has uncommitted changes; --worktree reads the policy from the commit at %s and does not read an uncommitted policy yet", path, head)
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
	scan := func(repo string) (report *codesignal.Report, stderr string, exitCode int) {
		stdout, stderrBytes, exitCode := runCoachCodesignalBaselineRaw(repo, "--worktree", "--fail-on-incomplete-coverage", "--format=json")
		return decodeCoachReport(stdout), string(stderrBytes), exitCode
	}

	// The selected policy is the canonical file when both are committed, so the
	// deprecated one is the policy the scan did not select.
	entries := []TableEntry{
		Entry("a committed deprecated policy beside the selected canonical one, edited",
			[]string{".lousy-agents/coach.json", "project.json"}, func(repo string) {
				overwriteOnDisk(repo, "project.json", goLayerPolicyConfigJSON)
			}, "project.json", hasChanges),
		Entry("a committed deprecated policy beside the selected canonical one, renamed away",
			[]string{".lousy-agents/coach.json", "project.json"}, func(repo string) {
				runGitInRepo(repo, "mv", "project.json", "moved.json")
			}, "project.json", hasChanges),
		Entry("a canonical policy staged beside the selected deprecated one",
			[]string{"project.json"}, func(repo string) {
				overwriteOnDiskCreatingParents(repo, ".lousy-agents/coach.json", permissivePolicyJSON)
				runGitInRepo(repo, "add", ".lousy-agents/coach.json")
			}, ".lousy-agents/coach.json", notCommitted),
		Entry("a canonical policy untracked beside the selected deprecated one",
			[]string{"project.json"}, func(repo string) {
				overwriteOnDiskCreatingParents(repo, ".lousy-agents/coach.json", permissivePolicyJSON)
			}, ".lousy-agents/coach.json", notCommitted),
	}

	DescribeTable("words the diagnostic by whether the commit holds the file",
		func(committed []string, change func(repo string), policyPath string, wording func(path, head string) string) {
			repo, head := repoCommitting(committed...)
			change(repo)

			report, stderr, exitCode := scan(repo)

			Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
			Expect(diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).To(ConsistOf(And(
				HaveField("Path", policyPath),
				HaveField("Message", wording(policyPath, head)),
			)))
		},
		entries,
	)

	DescribeTable("names it once, and does not also list it as a change the scan did not read",
		func(committed []string, change func(repo string), policyPath string, _ func(path, head string) string) {
			repo, _ := repoCommitting(committed...)
			change(repo)

			report, stderr, exitCode := scan(repo)

			Expect(exitCode).To(Equal(3), "stderr: %s", stderr)
			Expect(diagnosticsOfKind(report.Diagnostics, "worktree_changes_not_analyzed")).To(ConsistOf(HaveField("Path", policyPath)))
			Expect(diagnosticsOfKind(report.Diagnostics, "worktree_not_clean")).NotTo(ContainElement(HaveField("Message", ContainSubstring(policyPath))),
				"the policy already has its own diagnostic, so listing it again would count it twice")
		},
		entries,
	)
})
